package files

import (
	"crypto/md5"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func NewFileService(config *Config) *FileService {
	if config == nil {
		config = &Config{
			BaseDir:     "/tmp",
			MaxFileSize: 10 * 1024 * 1024, // 10 MB
		}
	}
	return &FileService{config: config}
}

func (u *FileService) ListDirectories() ([]Directory, error) {
	// List all directories on disk
	entries, err := os.ReadDir(u.config.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %v", err)
	}

	dirInfo := make([]Directory, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			info, err := os.Stat(filepath.Join(u.config.BaseDir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed to stat directory %s: %v", entry.Name(), err)
			}
			dirPath := filepath.Join(u.config.BaseDir, entry.Name())
			files, err := os.ReadDir(dirPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read directory %s: %v", dirPath, err)
			}
			fileCount := 0
			totalSize := int64(0)
			for _, file := range files {
				if !file.IsDir() {
					fileCount++
					fileInfo, err := file.Info()
					if err != nil {
						slog.Warn("Failed to get file info", "error", err)
						continue
					}
					totalSize += fileInfo.Size()
				}
			}

			dirInfo = append(dirInfo, Directory{
				Name:         entry.Name(),
				FileCount:    fileCount,
				Size:         totalSize,
				LastModified: info.ModTime(),
			})
		}

	}

	return dirInfo, nil
}

func (u *FileService) ListFiles(directory string) (*Directory, error) {
	// Validate the directory
	if directory == "" {
		return nil, fmt.Errorf("directory is required")
	}
	dirPath := filepath.Join(u.config.BaseDir, directory)

	// Get directory info
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory %s: %v", dirPath, err)
	}

	// List all files in the specified directory
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %v", dirPath, err)
	}

	fileList := make([]File, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			fileInfo, err := file.Info()
			if err != nil {
				return nil, fmt.Errorf("failed to get file info %s: %v", file.Name(), err)
			}
			fileList = append(fileList, File{
				Name:         fileInfo.Name(),
				DisplayName:  u.DisplayName(fileInfo.Name()),
				Size:         fileInfo.Size(),
				LastModified: fileInfo.ModTime(),
			})
		}
	}

	// Create a Directory struct to return
	dirInfo := &Directory{
		Name:         directory,
		FileCount:    len(fileList),
		Files:        fileList,
		Size:         info.Size(),
		LastModified: info.ModTime(),
	}

	return dirInfo, nil
}

func (u *FileService) GetFilePath(directory, filename string) (string, os.FileInfo, error) {
	// Validate the directory and filename
	if directory == "" || filename == "" {
		return "", nil, fmt.Errorf("directory and filename are required")
	}
	filePath := filepath.Join(u.config.BaseDir, directory, filename)
	filePath = filepath.Clean(filePath)

	// Validate the file
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return "", nil, fmt.Errorf("file does not exist")
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to stat file %s: %v", filePath, err)
	}
	if fileInfo.IsDir() {
		return "", nil, fmt.Errorf("path is a directory, not a file")
	}
	// Check file size
	if fileInfo.Size() > u.config.MaxFileSize {
		return "", nil, fmt.Errorf("file size exceeds the maximum allowed size")
	}

	return filePath, fileInfo, nil
}

func (u *FileService) DisplayName(filename string) string {
	return strings.SplitN(filename, "-", 3)[2]
}

func (u *FileService) Md5Sum(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to calculate MD5: %v", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
