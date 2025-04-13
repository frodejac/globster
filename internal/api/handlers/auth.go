package handlers

import (
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/auth/google"
	"github.com/frodejac/globster/internal/auth/static"
	"github.com/frodejac/globster/internal/config"
	"html/template"
	"log"
	"net/http"
)

type AuthHandler struct {
	BaseHandler
	googleAuth *google.Auth
	staticAuth *static.Auth
}

func NewAuthHandler(authType config.AuthType, sessions *auth.SessionService, templates *template.Template, googleAuth *google.Auth, staticAuth *static.Auth) *AuthHandler {
	return &AuthHandler{
		BaseHandler: BaseHandler{
			authType:  authType,
			sessions:  sessions,
			templates: templates,
		},
		googleAuth: googleAuth,
		staticAuth: staticAuth,
	}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && h.authType == config.AuthTypeGoogle {
		h.googleAuth.Redirect(w, r)
		return
	}
	if r.Method == http.MethodPost && h.authType == config.AuthTypeStatic {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		username := r.PostForm.Get("username")
		password := r.PostForm.Get("password")
		if ok := h.staticAuth.Validate(username, password); !ok {
			log.Printf("Invalid login attempt: %s", r.FormValue("username"))
			http.Redirect(w, r, "/?state=1", http.StatusFound)
			return
		}
		if _, err := h.sessions.Create(w); err != nil {
			log.Printf("Error creating session: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/home/", http.StatusFound)
	}
	// Invalid method/auth type combination
	log.Printf("Invalid login method or auth type (method: %s, auth type: %s)", r.Method, h.authType)
	h.render404(w)
}

func (h *AuthHandler) HandleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.authType != config.AuthTypeGoogle {
		h.render404(w)
		return
	}
	if err := h.googleAuth.Callback(w, r); err != nil {
		log.Printf("Google OAuth callback error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Set session cookie
	if _, err := h.sessions.Create(w); err != nil {
		log.Printf("Error creating session: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Redirect to admin home
	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Destroy(w, r); err != nil {
		log.Printf("Error destroying session: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
