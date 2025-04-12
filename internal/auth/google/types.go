package google

import (
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	admin "google.golang.org/api/admin/directory/v1"
)

type Config struct {
	AllowedDomains               []string
	AllowedGroups                []string
	Issuer                       string
	ClientID                     string
	ClientSecret                 string
	CookieSecure                 bool
	RedirectURL                  string
	ServiceAccountConfigJsonPath string
	Scopes                       []string
}

type Auth struct {
	adminService   *admin.Service
	allowedDomains []string
	allowedGroups  []string
	cookieSecure   bool
	oauthConfig    *oauth2.Config
	oidcProvider   *oidc.Provider
}

type UserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Domain  string `json:"hd"`
}
