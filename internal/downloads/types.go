package downloads

import "github.com/frodejac/globster/internal/database/links"

type Config struct {
	BaseDir string
}

type DownloadService struct {
	store  *links.Store
	config *Config
}
