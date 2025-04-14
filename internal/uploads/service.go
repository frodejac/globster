package uploads

import (
	"fmt"
	"github.com/frodejac/globster/internal/database/links"
	"github.com/frodejac/globster/internal/random"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func NewUploadService(store *links.Store, cfg *Config) (*UploadService, error) {
	uploads := &UploadService{
		store:  store,
		config: cfg,
	}
	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(cfg.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating upload directory: %v", err)
	}
	return uploads, nil
}

func (u *UploadService) CreateLink(directory string, expiresAt time.Time, remainingUses int) error {
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
	// Insert the upload link into the database
	if err := u.store.Create(token, directory, expiresAt, remainingUses); err != nil {
		return fmt.Errorf("failed to create upload link: %v", err)
	}
	return nil
}

func (u *UploadService) DeactivateLink(token string) error {
	// Validate the token
	if token == "" {
		return fmt.Errorf("token is required")
	}
	// Deactivate the upload link
	if err := u.store.Deactivate(token); err != nil {
		return fmt.Errorf("failed to deactivate upload link: %v", err)
	}
	return nil
}

func (u *UploadService) DeleteLink(token string) error {
	// Validate the token
	if token == "" {
		return fmt.Errorf("token is required")
	}
	// Delete the upload link
	if err := u.store.Delete(token); err != nil {
		return fmt.Errorf("failed to delete upload link: %v", err)
	}
	return nil
}

func (u *UploadService) ValidateToken(token string) (*links.Link, error) {
	// Validate the token
	if token == "" {
		return nil, fmt.Errorf("no token provided")
	}
	// Check if the token exists and is not expired
	link, err := u.store.Get(token)
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

func (u *UploadService) Upload(r *http.Request, link *links.Link) error {
	if err := r.ParseMultipartForm(u.config.MaxFileSize); err != nil {
		return fmt.Errorf("failed to parse form: %v", err)
	}

	// Get the file from the form
	file, handler, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("failed to get file from form: %v", err)
	}
	defer file.Close()

	if handler.Size <= 0 {
		return fmt.Errorf("file size is zero")
	}

	// Check extension
	if !u.checkFileExtension(handler.Filename) {
		return fmt.Errorf("file extension not allowed")
	}

	// Check reported MIME type
	if mime := handler.Header["Content-Type"]; !u.checkMimeType(mime) {
		return fmt.Errorf("MIME type not allowed")
	}

	// Check actual MIME type
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %v", err)
	}
	if n > 0 {
		if mimeType := http.DetectContentType(buffer[:n]); !u.checkMimeType([]string{mimeType}) {
			return fmt.Errorf("MIME type not allowed")
		}
	}

	// Create the directory if it doesn't exist
	dirPath := filepath.Join(u.config.BaseDir, link.Dir)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	// Sanitize the filename
	filename := sanitizeFilename(handler.Filename, link.Token)

	// Don't overwrite existing files (highly unlikely, but still)
	if _, err := os.Stat(filepath.Join(dirPath, filename)); err == nil {
		return fmt.Errorf("file already exists")
	}

	outfile, err := os.Create(filepath.Join(dirPath, filename))
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer outfile.Close()

	// Rewind the file reader to the beginning
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek file: %v", err)
	}

	if _, err := io.Copy(outfile, file); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	// Update the remaining uses and last used time in the database
	if err := u.store.Update(link.Token, link.RemainingUses-1, time.Now()); err != nil {
		return fmt.Errorf("failed to update remaining uses: %v", err)
	}

	return nil
}

func (u *UploadService) ListDirectories() ([]Directory, error) {
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
			for _, file := range files {
				if !file.IsDir() {
					fileCount++
				}
			}

			dirInfo = append(dirInfo, Directory{
				Name:         entry.Name(),
				FileCount:    fileCount,
				Size:         info.Size(),
				LastModified: info.ModTime(),
			})
		}

	}

	return dirInfo, nil
}

func (u *UploadService) ListFiles(directory string) (*Directory, error) {
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

func (u *UploadService) GetFilePath(directory, filename string) (string, os.FileInfo, error) {
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

func (u *UploadService) Download(w http.ResponseWriter, r *http.Request, directory, filename string) error {
	// Validate the directory and filename
	if directory == "" || filename == "" {
		return fmt.Errorf("directory and filename are required")
	}
	filePath := filepath.Join(u.config.BaseDir, directory, filename)

	// Check if the file exists
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file does not exist")
	}
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %v", filePath, err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file")
	}
	// Check file size
	if fileInfo.Size() > u.config.MaxFileSize {
		return fmt.Errorf("file size exceeds the maximum allowed size")
	}
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	http.ServeContent(w, r, filename, fileInfo.ModTime(), file)
	return nil
}

func (u *UploadService) checkFileExtension(filename string) bool {
	ext := filepath.Ext(filename)
	for _, allowedExt := range u.config.AllowedExtensions {
		if allowedExt == ext {
			return true
		}
	}
	return false
}

func (u *UploadService) checkMimeType(mime []string) bool {
	for _, allowedMime := range u.config.AllowedMimeTypes {
		for _, m := range mime {
			if strings.HasPrefix(m, allowedMime) {
				return true
			}
		}
	}
	return false
}

// sanitizeFilename sanitizes the filename by cleaning it up, extracting the base name,
// removing invalid characters, appending a random prefix, and ensuring it doesn't exceed
// the maximum length.
func sanitizeFilename(filename, token string) string {
	filename = filepath.Clean(filename)
	filename = filepath.Base(filename)
	filename = regexp.MustCompile(`[^a-zA-Z0-9\-_.]+`).ReplaceAllString(filename, "")
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	prefix := random.String(16)
	filename = fmt.Sprintf("%s-%s-%s%s", prefix, token, base, ext)
	if len(filename) > 255 {
		filename = filename[:255-len(ext)] + ext
	}
	return filename
}
