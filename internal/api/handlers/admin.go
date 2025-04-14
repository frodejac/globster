package handlers

import (
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/config"
	"github.com/frodejac/globster/internal/database/links"
	"github.com/frodejac/globster/internal/uploads"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type AdminHandler struct {
	BaseHandler
	baseUrl   string
	linkStore *links.Store
	uploads   *uploads.UploadService
}

func NewAdminHandler(authType config.AuthType, baseUrl string, sessions *auth.SessionService, templates *template.Template, linkStore *links.Store, uploads *uploads.UploadService) *AdminHandler {
	return &AdminHandler{
		BaseHandler: BaseHandler{
			authType:  authType,
			sessions:  sessions,
			templates: templates,
		},
		baseUrl:   baseUrl,
		linkStore: linkStore,
		uploads:   uploads,
	}
}

type AdminData struct {
	Links []links.Link
}

func (h *AdminHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	activeLinks, err := h.linkStore.ListActive()
	if err != nil {
		slog.Error("Failed to fetch active links", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for i := range activeLinks {
		activeLinks[i].Url = h.baseUrl + activeLinks[i].Url
	}

	h.renderTemplate(w, "admin_home.html", AdminData{Links: activeLinks})
}

func (h *AdminHandler) HandleCreateLink(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Get the parameters from the form
	directory := r.FormValue("directory")
	if directory == "" {
		http.Error(w, "Missing directory", http.StatusBadRequest)
		return
	}
	expiresInStr := r.FormValue("expiresIn")
	if expiresInStr == "" {
		http.Error(w, "Missing expiration", http.StatusBadRequest)
		return
	}
	var expiresAt time.Time
	expiresIn, err := time.ParseDuration(expiresInStr)
	if err != nil {
		http.Error(w, "Invalid expiration duration", http.StatusBadRequest)
		return
	}
	expiresAt = time.Now().Add(expiresIn)

	uses := r.FormValue("uses")
	if uses == "" {
		http.Error(w, "Missing remaining uses", http.StatusBadRequest)
		return
	}
	remainingUses, err := strconv.Atoi(uses)
	if err != nil {
		http.Error(w, "Invalid remaining uses", http.StatusBadRequest)
		return
	}
	if err := h.uploads.CreateLink(directory, expiresAt, remainingUses); err != nil {
		slog.Error("Failed to create upload link", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (h *AdminHandler) HandleDeactivateLink(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Get the token from the form
	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Deactivate the link in the database
	if err := h.uploads.DeactivateLink(token); err != nil {
		slog.Error("Failed to deactivate upload link", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}
