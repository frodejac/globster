package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/coreos/go-oidc/v3/oidc"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Application struct {
	Config             *Config
	DB                 *sql.DB
	Templates          *template.Template
	OAuth              *oauth2.Config
	OidcProvider       *oidc.Provider
	GoogleAdminService *admin.Service
	Users              map[string]string
}

// Modified main function to use FileServer
func main() {
	config := LoadConfig()
	app := &Application{
		Config: config,
	}

	if config.Auth.Type == AuthTypeGoogle {

		data, err := os.ReadFile(config.Auth.Google.ServiceAccountConfigJsonPath)
		if err != nil {
			log.Fatal(err)
		}

		jwtConfig, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/admin.directory.group.member.readonly")
		if err != nil {
			log.Fatalf("Failed to parse JWT config: %v", err)
		}

		adminService, err := admin.NewService(context.Background(), option.WithTokenSource(jwtConfig.TokenSource(context.Background())))
		if err != nil {
			log.Fatalf("Failed to create admin service: %v", err)
		}

		provider, err := oidc.NewProvider(context.Background(), config.Auth.Google.Issuer)
		if err != nil {
			log.Fatalf("oidc.NewProvider: %v", err)
		}
		oauthConfig := &oauth2.Config{
			ClientID:     config.Auth.Google.ClientID,
			ClientSecret: config.Auth.Google.ClientSecret,
			RedirectURL:  config.BaseURL + "/oauth/google/callback",
			Scopes:       config.Auth.Google.Scopes,
			Endpoint:     google.Endpoint,
		}
		app.GoogleAdminService = adminService
		app.OidcProvider = provider
		app.OAuth = oauthConfig
	}

	if config.Auth.Type == AuthTypeStatic {
		data, err := os.ReadFile(config.Auth.Static.UsersJsonPath)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(data, &app.Users); err != nil {
			log.Fatalf("Failed to unmarshal users: %v", err)
		}
		if len(app.Users) == 0 {
			log.Fatal("No users found in static auth config")
		}
		log.Printf("Loaded %d users from static auth config", len(app.Users))
	}

	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(config.Upload.Path, 0755); err != nil {
		log.Fatalf("Error creating upload directory: %v", err)
	}

	// Open database connection
	db, err := initDatabase(config.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	app.DB = db

	// Parse templates
	templates, err := template.ParseGlob(filepath.Join(config.TemplatePath, "*.html"))
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
	app.Templates = templates

	// Configure routes
	router := http.NewServeMux()

	// Public routes
	router.HandleFunc("/", app.homeHandler)
	router.HandleFunc("/login/", app.loginHandler)
	router.HandleFunc("GET /logout/", app.logoutHandler)
	router.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(config.StaticPath))))
	router.HandleFunc("GET /oauth/google/callback/", app.googleAuthCallbackHandler)
	router.HandleFunc("GET /upload/{token}/", app.getUploadHandler)
	router.HandleFunc("POST /upload/{token}/", app.postUploadHandler)
	router.HandleFunc("GET /upload/success/", func(w http.ResponseWriter, r *http.Request) { app.renderTemplate(w, "upload_success.html", nil) })
	router.HandleFunc("GET /upload/error/", func(w http.ResponseWriter, r *http.Request) { app.renderTemplate(w, "upload_error.html", nil) })

	// Admin routes (protected by middleware)
	adminRoutes := http.NewServeMux()
	adminRoutes.Handle("GET /admin/files/", http.StripPrefix("/admin/files/", http.FileServer(http.Dir(config.Upload.Path))))
	adminRoutes.HandleFunc("GET /admin/home/", app.adminHomeHandler)
	adminRoutes.HandleFunc("POST /admin/links/new/", app.createLinkHandler)
	adminRoutes.HandleFunc("POST /admin/links/deactivate/", app.deactivateLinkHandler)

	//loggedAdminRoutes := LoggingMiddleWare(adminRoutes)

	// Apply authentication middleware to admin routes
	router.Handle("/admin/", app.requireAuth(adminRoutes))

	loggedRouter := LoggingMiddleWare(router)

	// Start server
	log.Printf("Starting server on port %s", config.Server.Port)
	err = http.ListenAndServe(":"+config.Server.Port, loggedRouter)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func LoggingMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
