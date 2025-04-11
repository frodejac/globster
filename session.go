package main

import (
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/http"
	"time"
)

// Middleware to require authentication
func (app *Application) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionId, err := app.getSession(r)
		if err != nil {
			if sessionId != "" {
				// Session exists but is invalid, clear the cookie
				http.SetCookie(w, app.createSessionCookie(sessionId, time.Unix(0, 0)))
			}
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		// Session is valid, continue to the next handler
		next.ServeHTTP(w, r)
	})
}

func (app *Application) getSession(r *http.Request) (string, error) {
	// Get session ID from cookie
	sessionId, err := app.getSessionId(r)
	if err != nil {
		return "", err
	}

	var expiresAt time.Time
	err = app.DB.QueryRow(
		"SELECT expires_at FROM sessions WHERE id = ?",
		sessionId,
	).Scan(&expiresAt)

	if err != nil {
		return "", err
	}

	if expiresAt.Before(time.Now()) {
		// Session expired
		_, err = app.DB.Exec(
			"DELETE FROM sessions WHERE id = ?",
			sessionId,
		)
		if err != nil {
			log.Printf("Failed to delete expired session: %v", err)
		}
		return sessionId, fmt.Errorf("session expired")
	}
	return sessionId, nil
}

func (app *Application) createSession(w http.ResponseWriter) string {
	sessionID := uuid.New().String()
	if _, err := app.DB.Exec(
		"INSERT INTO sessions (id, created_at, expires_at) VALUES (?, ?, ?)",
		sessionID, time.Now(), time.Now().Add(app.Config.Session.Lifetime),
	); err != nil {
		log.Printf("Failed to create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
	}
	sessionCookie := app.createSessionCookie(sessionID, time.Now().Add(app.Config.Session.Lifetime))
	http.SetCookie(w, sessionCookie)
	log.Printf("Session created: %s", sessionID)
	return sessionID
}

func (app *Application) destroySession(w http.ResponseWriter, r *http.Request) {
	sessionId, err := app.getSessionId(r)
	if err != nil {
		// Session cookie not found, nothing to do
		log.Printf("Failed to get session ID from cookie: %v", err)
		return
	}
	if _, err := app.DB.Exec(
		"DELETE FROM sessions WHERE id = ?",
		sessionId,
	); err != nil {
		log.Printf("Failed to delete session: %v", err)
	}
	// Clear the session cookie
	http.SetCookie(w, app.createSessionCookie(sessionId, time.Unix(0, 0)))
	log.Printf("Session destroyed: %s", sessionId)
}

func (app *Application) createSessionCookie(sessionId string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     app.Config.Session.Cookie.Name,
		Value:    sessionId,
		Expires:  expires,
		Path:     app.Config.Session.Cookie.Path,
		HttpOnly: app.Config.Session.Cookie.HttpOnly,
		Secure:   app.Config.Session.Cookie.Secure,
		SameSite: app.Config.Session.Cookie.SameSite,
	}
}

func (app *Application) getSessionId(r *http.Request) (string, error) {
	cookie, err := r.Cookie(app.Config.Session.Cookie.Name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}
