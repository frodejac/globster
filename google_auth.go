package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// GoogleUserInfo represents the user information returned by Google
type GoogleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Domain  string `json:"hd"`
}

// Get user info from Google
func getGoogleUserInfo(client *http.Client) (*GoogleUserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(data, &userInfo); err != nil {
		return nil, err
	}
	return &userInfo, nil
}

func (app *Application) setAuthState(w http.ResponseWriter) string {
	state := uuid.New().String()

	// Store state in a cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Path:     "/",
		Secure:   app.Config.Session.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	return state
}

func (app *Application) getAuthState(r *http.Request) (string, error) {
	cookie, err := r.Cookie("oauthstate")
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

func (app *Application) clearAuthState(w http.ResponseWriter) {
	// Store state in a cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Path:     "/",
		Secure:   app.Config.Session.Cookie.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (app *Application) validateAuthState(r *http.Request) error {
	cookieState, err := app.getAuthState(r)
	formState := r.FormValue("state")
	if err != nil || cookieState != formState {
		return fmt.Errorf("invalid oauth state")
	}
	return nil
}

func (app *Application) exchangeToken(ctx context.Context, code string) (*oauth2.Token, error) {
	oauth2Token, err := app.OAuth.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	return oauth2Token, nil
}

func (app *Application) getGoogleUserInfo(ctx context.Context, oauth2Token *oauth2.Token) (*GoogleUserInfo, error) {
	client := app.OAuth.Client(ctx, oauth2Token)
	userinfo, err := getGoogleUserInfo(client)
	if err != nil {
		return nil, err
	}
	return userinfo, nil
}

func (app *Application) verifyDomain(userinfo *GoogleUserInfo) bool {
	if len(app.Config.Auth.Google.AllowedDomains) == 0 {
		// No domain restrictions, allow all
		return true
	}
	emailParts := strings.Split(userinfo.Email, "@")
	if len(emailParts) != 2 || !slices.Contains(app.Config.Auth.Google.AllowedDomains, emailParts[1]) {
		return false
	}
	return true
}

func (app *Application) verifyGroupMembership(userinfo *GoogleUserInfo) bool {
	if len(app.Config.Auth.Google.AllowedGroups) == 0 {
		// No group restrictions, allow all
		return true
	}
	for _, group := range app.Config.Auth.Google.AllowedGroups {
		if _, err := app.GoogleAdminService.Members.Get(group, userinfo.Email).Do(); err == nil {
			return true
		}
	}
	return false
}
