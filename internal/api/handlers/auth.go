package handlers

import (
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/auth/google"
	"github.com/frodejac/globster/internal/auth/static"
	"github.com/frodejac/globster/internal/config"
	"golang.org/x/time/rate"
	"html/template"
	"log/slog"
	"net/http"
)

type AuthHandler struct {
	BaseHandler
	googleAuth *google.Auth
	staticAuth *static.Auth
	limiter    *rate.Limiter
}

func NewAuthHandler(authType config.AuthType, rateLimit rate.Limit, sessions *auth.SessionService, templates *template.Template, googleAuth *google.Auth, staticAuth *static.Auth) *AuthHandler {
	return &AuthHandler{
		BaseHandler: BaseHandler{
			authType:  authType,
			sessions:  sessions,
			templates: templates,
		},
		googleAuth: googleAuth,
		staticAuth: staticAuth,
		limiter:    rate.NewLimiter(rateLimit, 1),
	}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && h.authType == config.AuthTypeGoogle {
		h.googleAuth.Redirect(w, r)
		return
	}
	if r.Method == http.MethodPost && h.authType == config.AuthTypeStatic {
		if !h.limiter.Allow() {
			slog.Warn("Rate limit exceeded", slog.String("username", r.PostForm.Get("username")))
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		username := r.PostForm.Get("username")
		password := r.PostForm.Get("password")
		if ok := h.staticAuth.Validate(username, password); !ok {
			slog.Warn("Invalid login attempt", slog.String("username", username))
			http.Redirect(w, r, "/?state=1", http.StatusFound)
			return
		}
		if _, err := h.sessions.Create(w); err != nil {
			slog.Error("Error creating session", slog.String("username", username), slog.Any("error", err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/home/", http.StatusFound)
		return
	}
	// Invalid method/auth type combination
	slog.Debug("Invalid login method or auth type", slog.String("method", r.Method), slog.String("auth_type", string(h.authType)))
	h.render404(w)
}

func (h *AuthHandler) HandleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if h.authType != config.AuthTypeGoogle {
		h.render404(w)
		return
	}
	if err := h.googleAuth.Callback(w, r); err != nil {
		slog.Error("Google OAuth callback error", slog.Any("error", err))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Set session cookie
	if _, err := h.sessions.Create(w); err != nil {
		slog.Error("Error creating session", slog.Any("error", err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Redirect to admin home
	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Destroy(w, r); err != nil {
		slog.Error("Error destroying session", slog.Any("error", err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Clear-Site-Data", "cookies")
	http.Redirect(w, r, "/", http.StatusFound)
}
