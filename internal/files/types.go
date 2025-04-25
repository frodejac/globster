package files

import "time"

type Config struct {
	BaseDir     string
	MaxFileSize int64
}

type FileService struct {
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
}
