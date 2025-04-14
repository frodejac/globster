package handlers

import (
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/config"
	"html/template"
	"log/slog"
	"net/http"
)

type HomeHandler struct {
	BaseHandler
}

type HomeData struct {
	GoogleAuth bool
	StaticAuth bool
	Incorrect  bool
}

func NewHomeHandler(authType config.AuthType, sessions *auth.SessionService, templates *template.Template) *HomeHandler {
	home := &HomeHandler{
		BaseHandler: BaseHandler{
			authType:  authType,
			sessions:  sessions,
			templates: templates,
		},
	}
	return home
}

func (h *HomeHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/" {
		h.render404(w)
		return
	}
	ok, err := h.sessions.Validate(r)
	if err != nil {
		slog.Error("Error validating session", "error", err)
	}
	if ok {
		http.Redirect(w, r, "/admin/home/", http.StatusFound)
		return
	}

	state := r.URL.Query().Get("state")
	data := HomeData{
		GoogleAuth: h.authType == config.AuthTypeGoogle,
		StaticAuth: h.authType == config.AuthTypeStatic,
		Incorrect:  state != "",
	}
	// Render the home page
	h.renderTemplate(w, "home.html", data)
}
