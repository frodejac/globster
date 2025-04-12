package google

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

func (a *Auth) setAuthState(w http.ResponseWriter) string {
	state := generateRandomString(16)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Path:     "/",
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return state
}

func (a *Auth) getAuthState(r *http.Request) (string, error) {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func (a *Auth) clearAuthState(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *Auth) validateAuthState(r *http.Request) error {
	cookieState, err := a.getAuthState(r)
	if err != nil {
		return err
	}
	formState := r.FormValue("state")
	if cookieState != formState {
		return fmt.Errorf("invalid oauth state")
	}
	return nil
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
