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
	"github.com/frodejac/globster/internal/downloads"
	"github.com/frodejac/globster/internal/files"
	"github.com/frodejac/globster/internal/uploads"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}
	logOptions := &slog.HandlerOptions{Level: cfg.Logger.Level}
	var logHandler slog.Handler
	if cfg.Logger.Format == config.LogFormatJSON {
		logHandler = slog.NewJSONHandler(os.Stdout, logOptions)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, logOptions)
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	var googleAuth *g.Auth
	if cfg.Auth.Type == config.AuthTypeGoogle {
		cfg.Auth.Google.RedirectURL = cfg.BaseUrl + "/oauth/callback"
		googleAuth, err = g.NewAuthFromConfig(cfg.Auth.Google)
		if err != nil {
			slog.Error("Failed to create Google auth", "error", err)
			os.Exit(1)
		}
	}

	var staticAuth *s.Auth
	if cfg.Auth.Type == config.AuthTypeStatic {
		staticAuth, err = s.NewAuthFromConfig(cfg.Auth.Static)
		if err != nil {
			slog.Error("Failed to create static auth", "error", err)
			os.Exit(1)
		}
	}

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	linkStore, err := links.NewLinkStore(db)
	if err != nil {
		slog.Error("Failed to create link store", "error", err)
		os.Exit(1)
	}

	sessionStore, err := sessions.NewSessionStore(db)
	if err != nil {
		slog.Error("Failed to create session store", "error", err)
		os.Exit(1)
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
		slog.Error("Failed to create upload service", "error", err)
		os.Exit(1)
	}

	downloadService := downloads.NewDownloadService(
		linkStore,
		&downloads.Config{
			BaseDir: cfg.Upload.Path,
		},
	)

	fileService := files.NewFileService(&files.Config{
		BaseDir:     cfg.Upload.Path,
		MaxFileSize: cfg.Upload.MaxFileSize,
	})

	templates, err := template.ParseGlob(filepath.Join(cfg.TemplatePath, "*.html"))
	if err != nil {
		slog.Error("Failed to parse templates", "error", err)
		os.Exit(1)
	}

	apiCfg := &api.Config{
		AuthType:            cfg.Auth.Type,
		BaseUrl:             cfg.BaseUrl,
		StaticAuthRateLimit: cfg.Auth.RateLimit,
		StaticPath:          cfg.StaticPath,
		UploadPath:          cfg.Upload.Path,
	}

	router := api.NewRouter(
		templates,
		sessionService,
		linkStore,
		staticAuth,
		googleAuth,
		uploadService,
		downloadService,
		fileService,
		apiCfg,
	)

	mux := http.NewServeMux()
	router.SetupRoutes(mux)

	// Add middleware
	handler := api.SecurityHeadersMiddleware(cfg.Server.UseHsts)(mux)
	handler = api.LoggingMiddleWare(handler)
	handler = api.RequestIdMiddleware(handler)

	slog.Info("Starting server", "port", cfg.Server.Port)
	err = http.ListenAndServe(":"+cfg.Server.Port, handler)
	if err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
