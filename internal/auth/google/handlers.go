package google

import (
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
)

func (a *Auth) Redirect(w http.ResponseWriter, r *http.Request) {
	// Generate the URL for the OAuth2 authorization request
	state := a.setAuthState(w)
	authURL := a.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (a *Auth) Callback(w http.ResponseWriter, r *http.Request) error {
	// Make sure we clean up the state cookie after processing
	defer a.clearAuthState(w)

	// Validate the OAuth state
	if err := a.validateAuthState(r); err != nil {
		return fmt.Errorf("failed to validate state: %v", err)
	}

	// Exchange the authorization code for an access token
	code := r.URL.Query().Get("code")
	token, err := a.oauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		return fmt.Errorf("failed to exchange token: %w", err)
	}

	// Use the token to get user info
	client := a.oauthConfig.Client(context.Background(), token)
	userInfo, err := a.getUserInfo(client)
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	// Verify email domain if required
	if !a.verifyDomain(userInfo) {
		return fmt.Errorf("unauthorized domain: %s", userInfo.Email)
	}

	// Verify group membership if required
	if !a.verifyGroupMembership(userInfo) {
		return fmt.Errorf("unauthorized group membership: %s", userInfo.Email)
	}

	// User is authorized
	return nil
}
