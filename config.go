package main

import (
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type GoogleAuthConfig struct {
	AllowedDomains               []string
	AllowedGroups                []string
	Issuer                       string
	ClientID                     string
	ClientSecret                 string
	ServiceAccountConfigJsonPath string
	Scopes                       []string
}

func (g *GoogleAuthConfig) Validate() error {
	errors := []string{}
	warnings := []string{}
	if g.ClientID == "" {
		errors = append(errors, "ClientID is required")
	}
	if g.ClientSecret == "" {
		errors = append(errors, "ClientSecret is required")
	}
	if g.ServiceAccountConfigJsonPath == "" {
		errors = append(errors, "ServiceAccountConfigJsonPath is required")
	}
	if g.Issuer == "" {
		errors = append(errors, "Issuer is required")
	}
	if len(g.AllowedDomains) == 0 {
		warnings = append(warnings, "AllowedDomains is empty, all domains will be allowed")
	}
	if len(g.AllowedGroups) == 0 {
		warnings = append(warnings, "AllowedGroups is empty, all groups will be allowed")
	}
	if len(g.Scopes) == 0 {
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

type AuthType string

const (
	AuthTypeStatic AuthType = "static"
	AuthTypeGoogle AuthType = "google"
)

type StaticAuthConfig struct {
	UsersJsonPath string
}

type AuthConfig struct {
	Type   AuthType
	Google *GoogleAuthConfig
	Static *StaticAuthConfig
}

type ServerConfig struct {
	Port string
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
	BaseURL       string
	StaticPath    string
	TemplatePath  string
	IsDevelopment bool
	Server        *ServerConfig
	Database      *DatabaseConfig
	Session       *SessionConfig
	Upload        *UploadConfig
	Auth          *AuthConfig
}

func LoadConfig() *Config {
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
		log.Fatalf("Invalid AUTH_TYPE: %s", authTypeStr)
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
	httpsEnabled := os.Getenv("HTTPS_ENABLED")
	if httpsEnabled == "" {
		httpsEnabled = "false"
	}

	maxFileSizeStr := os.Getenv("MAX_FILE_SIZE_BYTES")
	if maxFileSizeStr == "" {
		maxFileSizeStr = "10485760" // 10 MB
	}
	maxFileSize, err := strconv.ParseInt(maxFileSizeStr, 10, 64)
	if err != nil {
		log.Fatalf("strconv.ParseInt: %v", err)
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

	googleAuth := &GoogleAuthConfig{
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
			log.Fatalf("googleAuth.Validate: %v", err)
		}
	}
	server := &ServerConfig{
		Port: serverPort,
	}
	database := &DatabaseConfig{
		Path: databasePath,
	}
	auth := &AuthConfig{
		Type:   authType,
		Google: googleAuth,
		Static: &StaticAuthConfig{
			UsersJsonPath: staticAuthPath,
		},
	}

	sessionLifetime, err := time.ParseDuration(sessionLifetimeStr)
	if err != nil {
		log.Fatalf("time.ParseDuration: %v", err)
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
	return &Config{
		BaseURL:       baseURL,
		StaticPath:    staticPath,
		TemplatePath:  templatePath,
		IsDevelopment: isDevelopment,
		Server:        server,
		Database:      database,
		Session:       session,
		Upload:        upload,
		Auth:          auth,
	}
}
