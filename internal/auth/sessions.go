package auth

import (
	"errors"
	"fmt"
	"github.com/frodejac/globster/internal/database/sessions"
	"github.com/frodejac/globster/internal/random"
	"net/http"
	"time"
)

type SessionCookieConfig struct {
	Name     string
	Path     string
	HttpOnly bool
	Secure   bool
	SameSite http.SameSite
	Lifetime time.Duration
}

type SessionService struct {
	store  *sessions.Store
	cookie *SessionCookieConfig
}

func NewSessionService(store *sessions.Store, cookieConfig *SessionCookieConfig) *SessionService {
	return &SessionService{
		store:  store,
		cookie: cookieConfig,
	}
}

func (s *SessionService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		valid, err := s.Validate(r)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if !valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *SessionService) Validate(r *http.Request) (bool, error) {
	id, err := s.getSessionId(r)
	if err != nil {
		return false, fmt.Errorf("failed to get session ID: %w", err)
	}
	if id == "" {
		return false, nil
	}

	// Check if session exists
	session, err := s.store.Get(id)
	if err != nil {
		return false, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return false, nil
	}
	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		// Cleanup
		if err := s.store.Delete(id); err != nil {
			return false, fmt.Errorf("failed to delete expired session: %w", err)
		}
		return false, nil
	}
	// Session is valid
	return true, nil
}

func (s *SessionService) Create(w http.ResponseWriter) (string, error) {
	id := random.String(32)
	expiresAt := time.Now().Add(s.cookie.Lifetime)
	if err := s.store.Create(id, time.Now(), expiresAt); err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	cookie := &http.Cookie{
		Name:     s.cookie.Name,
		Value:    id,
		Expires:  expiresAt,
		Path:     s.cookie.Path,
		HttpOnly: s.cookie.HttpOnly,
		Secure:   s.cookie.Secure,
		SameSite: s.cookie.SameSite,
	}
	http.SetCookie(w, cookie)
	return id, nil
}

func (s *SessionService) Destroy(w http.ResponseWriter, r *http.Request) error {
	id, err := s.getSessionId(r)
	if err != nil {
		return fmt.Errorf("failed to get session ID: %w", err)
	}
	if id == "" {
		return nil // No session to destroy
	}
	if err := s.store.Delete(id); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	cookie := &http.Cookie{
		Name:     s.cookie.Name,
		Value:    id,
		Expires:  time.Unix(0, 0),
		Path:     s.cookie.Path,
		HttpOnly: s.cookie.HttpOnly,
		Secure:   s.cookie.Secure,
		SameSite: s.cookie.SameSite,
	}
	http.SetCookie(w, cookie)

	return nil
}

func (s *SessionService) getSessionId(r *http.Request) (string, error) {
	// Get session ID from cookie
	if s.cookie == nil {
		panic(fmt.Errorf("cookie config is nil"))
	}
	sess, err := r.Cookie(s.cookie.Name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get session cookie: %w", err)
	}
	id := sess.Value
	if id == "" {
		return "", nil
	}
	return id, nil
}
