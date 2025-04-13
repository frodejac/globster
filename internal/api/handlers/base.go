package handlers

import (
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/config"
	"html/template"
	"net/http"
)

type BaseHandler struct {
	authType  config.AuthType
	sessions  *auth.SessionService
	templates *template.Template
}

func (b *BaseHandler) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	// Execute the template
	if err := b.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (b *BaseHandler) render404(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	b.renderTemplate(w, "404.html", nil)
}
