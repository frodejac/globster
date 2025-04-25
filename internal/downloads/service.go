package downloads

import (
	"fmt"
	"github.com/frodejac/globster/internal/database/links"
	"github.com/frodejac/globster/internal/random"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

func NewDownloadService(store *links.Store, config *Config) *DownloadService {
	return &DownloadService{
		store:  store,
		config: config,
	}
}

func (u *DownloadService) CreateLink(directory string, expiresAt time.Time, remainingUses int) error {
	// Input validation
	if directory == "" {
		return fmt.Errorf("directory is required")
	}
	if expiresAt.IsZero() || expiresAt.Before(time.Now()) {
		return fmt.Errorf("expiration date is required and must be in the future")
	}
	if remainingUses <= 0 {
		return fmt.Errorf("remaining uses must be greater than 0")
	}
	// Create a new upload token
	token := random.String(32)

	// Sanitize directory
	directory = filepath.Clean(directory)
	directory = filepath.Base(directory)
	directory = regexp.MustCompile("[^a-zA-Z0-9\\-_]+").ReplaceAllString(directory, "")

	if directory == "" {
		return fmt.Errorf("invalid directory name")
	}

	// Check that the directory exists
	dirPath := filepath.Join(u.config.BaseDir, directory)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist")
	}

	// Insert the download link into the database
	if err := u.store.CreateDownloadLink(token, directory, expiresAt, remainingUses); err != nil {
		return fmt.Errorf("failed to create upload link: %v", err)
	}
	return nil
}

func (u *DownloadService) DeactivateLink(token string) error {
	// Validate the token
	if token == "" {
		return fmt.Errorf("token is required")
	}
	// Deactivate the upload link
	if err := u.store.DeactivateDownloadLink(token); err != nil {
		return fmt.Errorf("failed to deactivate upload link: %v", err)
	}
	return nil
}

func (u *DownloadService) DeleteLink(token string) error {
	// Validate the token
	if token == "" {
		return fmt.Errorf("token is required")
	}
	// Delete the upload link
	if err := u.store.DeleteDownloadLink(token); err != nil {
		return fmt.Errorf("failed to delete upload link: %v", err)
	}
	return nil
}

func (u *DownloadService) ValidateToken(token string) (*links.DownloadLink, error) {
	// Validate the token
	if token == "" {
		return nil, fmt.Errorf("no token provided")
	}
	// Check if the token exists and is not expired
	link, err := u.store.GetDownloadLink(token)
	if err != nil {
		return nil, fmt.Errorf("failed to get upload link: %v", err)
	}
	if link.RemainingUses <= 0 {
		return nil, fmt.Errorf("token exhausted")
	}
	if link.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}
	return link, nil
}
