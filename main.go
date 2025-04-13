package main

import (
	"database/sql"
	g "github.com/frodejac/globster/internal/auth/google"
	s "github.com/frodejac/globster/internal/auth/static"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Application struct {
	Config     *Config
	DB         *sql.DB
	Templates  *template.Template
	Users      map[string]string
	GoogleAuth *g.Auth
	StaticAuth *s.Auth
}

func main() {
	config := LoadConfig()
	app := &Application{
		Config: config,
	}

	if config.Auth.Type == AuthTypeGoogle {
		config.Auth.Google.RedirectURL = config.BaseURL + "/oauth/callback"
		googleAuth, err := g.NewAuthFromConfig(config.Auth.Google)
		if err != nil {
			log.Fatalf("Failed to create Google auth: %v", err)
		}
		app.GoogleAuth = googleAuth
	}

	if config.Auth.Type == AuthTypeStatic {
		staticAuth, err := s.NewAuthFromConfig(config.Auth.Static)
		if err != nil {
			log.Fatalf("Failed to create static auth: %v", err)
		}
		app.StaticAuth = staticAuth
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
	router.HandleFunc("GET /oauth/callback/", app.googleAuthCallbackHandler)
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
