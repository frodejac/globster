package uploads

import (
	"github.com/frodejac/globster/internal/database/links"
	"time"
)

type Config struct {
	MaxFileSize       int64
	BaseDir           string
	AllowedExtensions []string
	AllowedMimeTypes  []string
}

type UploadService struct {
	store  *links.Store
	config *Config
}

type Directory struct {
	Name         string
	Size         int64
	Files        []File
	FileCount    int
	LastModified time.Time
}

type File struct {
	Name         string
	DisplayName  string
	Size         int64
	LastModified time.Time
	//DownloadLink string
}
