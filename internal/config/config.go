package config

import (
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/frodejac/globster/internal/auth/google"
	"github.com/frodejac/globster/internal/auth/static"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type AuthType string

const (
	AuthTypeStatic AuthType = "static"
	AuthTypeGoogle AuthType = "google"
)

type AuthConfig struct {
	Type   AuthType
	Google *google.Config
	Static *static.Config
}

type ServerConfig struct {
	Port               string
	UseHsts            bool
	UseSecurityHeaders bool
}

type DatabaseConfig struct {
	Path string
}

type SessionCookieConfig struct {
	Name     string
	Path     string
	HttpOnly bool
	Secure   bool
	SameSite http.SameSite
}

type SessionConfig struct {
	Lifetime time.Duration
	Cookie   *SessionCookieConfig
}

type UploadConfig struct {
	Path              string
	MaxFileSize       int64
	AllowedMimeTypes  []string
	AllowedExtensions []string
}

type Config struct {
	BaseUrl       string
	StaticPath    string
	TemplatePath  string
	IsDevelopment bool
	Server        *ServerConfig
	Database      *DatabaseConfig
	Session       *SessionConfig
	Upload        *UploadConfig
	Auth          *AuthConfig
}

func LoadConfig() (*Config, error) {
	allowedDomainsStr := os.Getenv("ALLOWED_DOMAINS")
	if allowedDomainsStr == "" {
		allowedDomainsStr = "*"
	}
	allowedDomains := []string{}
	if allowedDomainsStr != "*" {
		allowedDomains = strings.Split(allowedDomainsStr, ",")
	}
	allowedGroupsStr := os.Getenv("ALLOWED_GROUPS")
	if allowedGroupsStr == "" {
		allowedGroupsStr = "*"
	}
	allowedGroups := []string{}
	if allowedGroupsStr != "*" {
		allowedGroups = strings.Split(allowedGroupsStr, ",")
	}

	allowedMimeTypesStr := os.Getenv("ALLOWED_MIME_TYPES")
	if allowedMimeTypesStr == "" {
		allowedMimeTypesStr = "text/plain"
	}
	allowedMimeTypes := strings.Split(allowedMimeTypesStr, ",")

	allowedExtensionsStr := os.Getenv("ALLOWED_EXTENSIONS")
	if allowedExtensionsStr == "" {
		allowedExtensionsStr = ".txt"
	}
	allowedExtensions := strings.Split(allowedExtensionsStr, ",")
	authTypeStr := os.Getenv("AUTH_TYPE")
	if authTypeStr == "" {
		authTypeStr = "static"
	}
	authType := AuthType(authTypeStr)
	if authType != AuthTypeStatic && authType != AuthTypeGoogle {
		return nil, fmt.Errorf("invalid AUTH_TYPE: %s", authTypeStr)
	}

	baseURL := os.Getenv("BASE_URL")
	cookieSecure := os.Getenv("COOKIE_SECURE") == "true"
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		databasePath = "globster.db"
	}
	isDevelopment := os.Getenv("ENVIRONMENT") == "development"
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleServiceAccountConfigJsonPath := os.Getenv("GOOGLE_SERVICE_ACCOUNT_CONFIG_JSON_PATH")
	host := os.Getenv("HOST")
	if host == "" {
		host = "localhost"
	}

	maxFileSizeStr := os.Getenv("MAX_FILE_SIZE_BYTES")
	if maxFileSizeStr == "" {
		maxFileSizeStr = "10485760" // 10 MB
	}
	maxFileSize, err := strconv.ParseInt(maxFileSizeStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MAX_FILE_SIZE_BYTES: %v", err)
	}

	scopes := os.Getenv("SCOPES")
	if scopes == "" {
		scopes = fmt.Sprintf(
			"%s %s %s",
			oidc.ScopeOpenID,
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		)
	}
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}
	serverUseHstsStr := os.Getenv("USE_HSTS")
	if serverUseHstsStr == "" {
		serverUseHstsStr = "false"
	}
	serverUseHsts, err := strconv.ParseBool(serverUseHstsStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse USE_HSTS: %v", err)
	}
	serverUseSecurityHeadersStr := os.Getenv("USE_SECURITY_HEADERS")
	if serverUseSecurityHeadersStr == "" {
		serverUseSecurityHeadersStr = "false"
	}
	serverUseSecurityHeaders, err := strconv.ParseBool(serverUseSecurityHeadersStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse USE_SECURITY_HEADERS: %v", err)
	}
	sessionLifetimeStr := os.Getenv("SESSION_LIFETIME")
	if sessionLifetimeStr == "" {
		sessionLifetimeStr = "8h"
	}
	staticAuthPath := os.Getenv("STATIC_AUTH_PATH")
	if staticAuthPath == "" {
		staticAuthPath = "users.json"
	}
	staticPath := os.Getenv("STATIC_PATH")
	if staticPath == "" {
		staticPath = "web/static"
	}
	templatePath := os.Getenv("TEMPLATE_PATH")
	if templatePath == "" {
		templatePath = "web/templates"
	}
	uploadPath := os.Getenv("UPLOAD_PATH")
	if uploadPath == "" {
		uploadPath = "uploads"
	}

	googleAuth := &google.Config{
		AllowedDomains:               allowedDomains,
		AllowedGroups:                allowedGroups,
		Issuer:                       "https://accounts.google.com",
		ClientID:                     googleClientID,
		ClientSecret:                 googleClientSecret,
		ServiceAccountConfigJsonPath: googleServiceAccountConfigJsonPath,
		Scopes:                       strings.Split(scopes, " "),
	}
	if authType == AuthTypeGoogle {
		if err := googleAuth.Validate(); err != nil {
			return nil, fmt.Errorf("failed to validate Google auth config: %v", err)
		}
	}
	server := &ServerConfig{
		Port:               serverPort,
		UseHsts:            serverUseHsts,
		UseSecurityHeaders: serverUseSecurityHeaders,
	}
	database := &DatabaseConfig{
		Path: databasePath,
	}
	auth := &AuthConfig{
		Type:   authType,
		Google: googleAuth,
		Static: &static.Config{
			UsersJsonPath: staticAuthPath,
		},
	}

	sessionLifetime, err := time.ParseDuration(sessionLifetimeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SESSION_LIFETIME: %v", err)
	}
	session := &SessionConfig{
		Lifetime: sessionLifetime,
		Cookie: &SessionCookieConfig{
			Name:     "session",
			Path:     "/",
			HttpOnly: true,
			Secure:   cookieSecure,
			SameSite: http.SameSiteLaxMode,
		},
	}

	upload := &UploadConfig{
		Path:              uploadPath,
		MaxFileSize:       maxFileSize,
		AllowedMimeTypes:  allowedMimeTypes,
		AllowedExtensions: allowedExtensions,
	}

	if baseURL == "" {
		baseURL = "http://localhost:" + serverPort
	}
	cfg := &Config{
		BaseUrl:       baseURL,
		StaticPath:    staticPath,
		TemplatePath:  templatePath,
		IsDevelopment: isDevelopment,
		Server:        server,
		Database:      database,
		Session:       session,
		Upload:        upload,
		Auth:          auth,
	}
	return cfg, nil
}
