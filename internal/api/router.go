package api

import (
	h "github.com/frodejac/globster/internal/api/handlers"
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/auth/google"
	"github.com/frodejac/globster/internal/auth/static"
	"github.com/frodejac/globster/internal/config"
	"github.com/frodejac/globster/internal/database/links"
	"github.com/frodejac/globster/internal/downloads"
	"github.com/frodejac/globster/internal/files"
	"github.com/frodejac/globster/internal/uploads"
	"golang.org/x/time/rate"
	"html/template"
	"net/http"
)

type Config struct {
	AuthType            config.AuthType
	StaticAuthRateLimit rate.Limit
	BaseUrl             string
	StaticPath          string
	UploadPath          string
}

type handlers struct {
	admin    *h.AdminHandler
	auth     *h.AuthHandler
	home     *h.HomeHandler
	upload   *h.UploadHandler
	download *h.DownloadHandler
}

type Router struct {
	sessions *auth.SessionService
	config   *Config
	handlers *handlers
}

func NewRouter(
	templates *template.Template,
	sessions *auth.SessionService,
	links *links.Store,
	staticAuth *static.Auth,
	googleAuth *google.Auth,
	uploadService *uploads.UploadService,
	downloadService *downloads.DownloadService,
	fileService *files.FileService,
	config *Config,
) *Router {
	router := &Router{
		config: config,
		handlers: &handlers{
			admin:    h.NewAdminHandler(config.AuthType, config.BaseUrl, sessions, templates, links, uploadService, downloadService, fileService),
			auth:     h.NewAuthHandler(config.AuthType, config.StaticAuthRateLimit, sessions, templates, googleAuth, staticAuth),
			home:     h.NewHomeHandler(config.AuthType, sessions, templates),
			upload:   h.NewUploadHandler(config.AuthType, sessions, templates, uploadService),
			download: h.NewDownloadHandler(config.AuthType, sessions, templates, downloadService, fileService),
		},
		sessions: sessions,
	}
	return router
}

func (r *Router) SetupRoutes(mux *http.ServeMux) {
	// Public routes
	mux.HandleFunc("/{$}", r.handlers.home.HandleHome)
	mux.HandleFunc("/login", r.handlers.auth.HandleLogin)
	mux.HandleFunc("GET /logout", r.handlers.auth.HandleLogout)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(r.config.StaticPath))))
	mux.HandleFunc("GET /oauth/callback", r.handlers.auth.HandleGoogleOAuthCallback)
	mux.HandleFunc("GET /upload/{token}", r.handlers.upload.HandleGetUpload)
	mux.HandleFunc("POST /upload/{token}", r.handlers.upload.HandlePostUpload)
	mux.HandleFunc("GET /upload/success", r.handlers.upload.HandleSuccess)
	mux.HandleFunc("GET /upload/error", r.handlers.upload.HandleError)
	mux.HandleFunc("GET /download/{token}/{$}", r.handlers.download.HandleGetDirectory)
	mux.HandleFunc("GET /download/{token}/{file}", r.handlers.download.HandleGetFile)

	// Admin routes
	adminRoutes := http.NewServeMux()
	adminRoutes.HandleFunc("GET /admin/files/{$}", r.handlers.admin.HandleListDirectories)
	adminRoutes.HandleFunc("GET /admin/files/{directory}/{$}", r.handlers.admin.HandleListDirectory)
	adminRoutes.HandleFunc("GET /admin/files/{directory}/{filename}", r.handlers.admin.HandleDownloadFile)
	adminRoutes.HandleFunc("POST /admin/files/{directory}/share", r.handlers.admin.HandleShareDirectory)
	adminRoutes.HandleFunc("POST /admin/files/{directory}/unshare", r.handlers.admin.HandleUnshareDirectory)
	adminRoutes.HandleFunc("POST /admin/files/{directory}/upload", r.handlers.admin.HandlePostUpload)
	adminRoutes.HandleFunc("GET /admin/home/{$}", r.handlers.admin.HandleHome)
	adminRoutes.HandleFunc("POST /admin/links/new", r.handlers.admin.HandleCreateLink)
	adminRoutes.HandleFunc("POST /admin/links/deactivate", r.handlers.admin.HandleDeactivateLink)

	mux.Handle("/admin/", r.sessions.RequireAuth(adminRoutes))
}
