package google

import (
	"context"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
	"os"
	"strings"
)

func (c *Config) Validate() error {
	var errors, warnings []string
	if c.ClientID == "" {
		errors = append(errors, "ClientID is required")
	}
	if c.ClientSecret == "" {
		errors = append(errors, "ClientSecret is required")
	}
	if c.ServiceAccountConfigJsonPath == "" {
		errors = append(errors, "ServiceAccountConfigJsonPath is required")
	}
	if c.Issuer == "" {
		errors = append(errors, "Issuer is required")
	}
	if len(c.AllowedDomains) == 0 {
		warnings = append(warnings, "AllowedDomains is empty, all domains will be allowed")
	}
	if len(c.AllowedGroups) == 0 {
		warnings = append(warnings, "AllowedGroups is empty, all groups will be allowed")
	}
	if len(c.Scopes) == 0 {
		warnings = append(warnings, "Scopes is empty")
	}
	if len(errors) > 0 {
		return fmt.Errorf("configuration errors: %s", strings.Join(errors, ", "))
	}
	if len(warnings) > 0 {
		fmt.Printf("configuration warnings: %s\n", strings.Join(warnings, ", "))
	}
	return nil
}

func NewAuthFromConfig(config *Config) (*Auth, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(config.ServiceAccountConfigJsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account config: %v", err)
	}

	jwtConfig, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/admin.directory.group.member.readonly")
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT config: %v", err)
	}

	adminService, err := admin.NewService(context.Background(), option.WithTokenSource(jwtConfig.TokenSource(context.Background())))
	if err != nil {
		return nil, fmt.Errorf("failed to create admin service: %v", err)
	}

	provider, err := oidc.NewProvider(context.Background(), config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create oidc provider: %v", err)
	}

	auth := &Auth{
		adminService:   adminService,
		allowedDomains: config.AllowedDomains,
		allowedGroups:  config.AllowedGroups,
		cookieSecure:   config.CookieSecure,
		oauthConfig: &oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			RedirectURL:  config.RedirectURL,
			Scopes:       config.Scopes,
			Endpoint:     google.Endpoint,
		},
		oidcProvider: provider,
	}

	return auth, nil
}
