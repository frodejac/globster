package main

import (
	"github.com/frodejac/globster/internal/api"
	"github.com/frodejac/globster/internal/auth"
	g "github.com/frodejac/globster/internal/auth/google"
	s "github.com/frodejac/globster/internal/auth/static"
	"github.com/frodejac/globster/internal/config"
	"github.com/frodejac/globster/internal/database"
	"github.com/frodejac/globster/internal/database/links"
	"github.com/frodejac/globster/internal/database/sessions"
	"github.com/frodejac/globster/internal/uploads"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var googleAuth *g.Auth
	if cfg.Auth.Type == config.AuthTypeGoogle {
		cfg.Auth.Google.RedirectURL = cfg.BaseUrl + "/oauth/callback"
		googleAuth, err = g.NewAuthFromConfig(cfg.Auth.Google)
		if err != nil {
			log.Fatalf("Failed to create Google auth: %v", err)
		}
	}

	var staticAuth *s.Auth
	if cfg.Auth.Type == config.AuthTypeStatic {
		staticAuth, err = s.NewAuthFromConfig(cfg.Auth.Static)
		if err != nil {
			log.Fatalf("Failed to create static auth: %v", err)
		}
	}

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	linkStore, err := links.NewLinkStore(db)
	if err != nil {
		log.Fatalf("Failed to create link store: %v", err)
	}

	sessionStore, err := sessions.NewSessionStore(db)
	if err != nil {
		log.Fatalf("Failed to create session store: %v", err)
	}

	sessionCookieCfg := &auth.SessionCookieConfig{
		Name:     cfg.Session.Cookie.Name,
		Path:     cfg.Session.Cookie.Path,
		HttpOnly: cfg.Session.Cookie.HttpOnly,
		Secure:   cfg.Session.Cookie.Secure,
		SameSite: cfg.Session.Cookie.SameSite,
		Lifetime: cfg.Session.Lifetime,
	}

	sessionService := auth.NewSessionService(
		sessionStore,
		sessionCookieCfg,
	)

	uploadService, err := uploads.NewUploadService(
		linkStore,
		&uploads.Config{
			MaxFileSize:       cfg.Upload.MaxFileSize,
			BaseDir:           cfg.Upload.Path,
			AllowedExtensions: cfg.Upload.AllowedExtensions,
			AllowedMimeTypes:  cfg.Upload.AllowedMimeTypes,
		})
	if err != nil {
		log.Fatalf("Failed to create upload service: %v", err)
	}

	templates, err := template.ParseGlob(filepath.Join(cfg.TemplatePath, "*.html"))
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	apiCfg := &api.Config{
		AuthType:   cfg.Auth.Type,
		BaseUrl:    cfg.BaseUrl,
		StaticPath: cfg.StaticPath,
		UploadPath: cfg.Upload.Path,
	}

	router := api.NewRouter(
		templates,
		sessionService,
		linkStore,
		staticAuth,
		googleAuth,
		uploadService,
		apiCfg,
	)

	mux := http.NewServeMux()
	router.SetupRoutes(mux)

	log.Printf("Starting server on port %s", cfg.Server.Port)
	err = http.ListenAndServe(":"+cfg.Server.Port, api.LoggingMiddleWare(mux))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
