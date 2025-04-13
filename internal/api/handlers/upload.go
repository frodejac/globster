package handlers

import (
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/config"
	"github.com/frodejac/globster/internal/uploads"
	"html/template"
	"log"
	"net/http"
)

type UploadHandler struct {
	BaseHandler
	uploads *uploads.UploadService
}

type UploadData struct {
	Token string
}

func NewUploadHandler(authType config.AuthType, sessions *auth.SessionService, templates *template.Template, uploads *uploads.UploadService) *UploadHandler {
	return &UploadHandler{
		BaseHandler: BaseHandler{
			authType:  authType,
			sessions:  sessions,
			templates: templates,
		},
		uploads: uploads,
	}
}

func (h *UploadHandler) HandleGetUpload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if _, err := h.uploads.ValidateToken(token); err != nil {
		log.Printf("Invalid token: %s", token)
		h.render404(w)
		return
	}
	h.renderTemplate(w, "upload.html", UploadData{Token: token})
}

func (h *UploadHandler) HandlePostUpload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	link, err := h.uploads.ValidateToken(token)
	if err != nil {
		log.Printf("Invalid token: %s", token)
		h.render404(w)
		return
	}
	if err := h.uploads.Upload(r, link); err != nil {
		log.Printf("Upload error: %v", err)
		http.Redirect(w, r, "/upload/error/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/upload/success/", http.StatusFound)
}

func (h *UploadHandler) HandleSuccess(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "upload_success.html", nil)
}

func (h *UploadHandler) HandleError(w http.ResponseWriter, r *http.Request) {
	h.renderTemplate(w, "upload_error.html", nil)
}
