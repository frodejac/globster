package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func (app *Application) deactivateUploadLink(token string) error {
	// Validate the token
	if token == "" {
		return fmt.Errorf("token is required")
	}
	// Deactivate the upload link
	if _, err := app.DB.Exec(
		"UPDATE upload_links SET expires_at = ?, remaining_uses = 0 WHERE token = ?",
		time.Now(),
		token,
	); err != nil {
		return fmt.Errorf("failed to deactivate upload link: %v", err)
	}
	return nil
}

func (app *Application) createUploadLink(directory string, expiresAt time.Time, remainingUses int) error {
	// Validate the parameters
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
	token := generateRandomString(32)

	// Insert the upload link into the database
	if _, err := app.DB.Exec(
		"INSERT INTO upload_links (token, dir, expires_at, remaining_uses, created_at) VALUES (?, ?, ?, ?, ?)",
		token,
		directory,
		expiresAt,
		remainingUses,
		time.Now(),
	); err != nil {
		return fmt.Errorf("failed to create upload link: %v", err)
	}

	return nil
}

func (app *Application) validateUploadToken(r *http.Request) (string, error) {
	// Get the token from the request
	token := r.PathValue("token")
	if token == "" {
		return "", fmt.Errorf("no token provided")
	}

	// Validate the token
	// Check if the token exists and is not expired
	var expiresAt time.Time
	var remainingUses int
	if err := app.DB.QueryRow(
		"SELECT expires_at, remaining_uses FROM upload_links WHERE token = ? ",
		token,
	).Scan(&expiresAt, &remainingUses); err != nil {
		log.Printf("Token not found: %s; error: %v", token, err)
		return "", fmt.Errorf("token not found")
	}
	if expiresAt.Before(time.Now()) {
		log.Printf("Token expired: %s; expired %s", token, expiresAt)
		return "", fmt.Errorf("token expired")
	}
	if remainingUses <= 0 {
		log.Printf("Token exhausted: %s; remaining usages %d", token, remainingUses)
		return "", fmt.Errorf("token exhausted")
	}
	// Token is valid
	return token, nil
}

func (app *Application) upload(r *http.Request, token string) error {
	if err := r.ParseMultipartForm(app.Config.Upload.MaxFileSize); err != nil {
		return err
	}

	// Get the file from the form
	file, handler, err := r.FormFile("file")

	if err != nil {
		return err
	}
	defer file.Close()
	log.Printf("Uploaded file: %s (%d bytes)", handler.Filename, handler.Size)

	// Check file size
	if handler.Size > app.Config.Upload.MaxFileSize {
		return fmt.Errorf("file size exceeds limit")
	}
	// Check extension
	if ext := filepath.Ext(handler.Filename); !app.validateExtension(ext) {
		return fmt.Errorf("invalid file extension: %s", ext)
	}
	// Check reported MIME type
	if mime := handler.Header["Content-Type"]; !app.validateMimeType(mime) {
		log.Print("reported MIME type: ", mime)
		return fmt.Errorf("invalid MIME type: %s", mime)
	}

	// Check actual MIME type
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %v", err)
	}
	if mimeType := http.DetectContentType(buffer[:n]); !app.validateMimeType([]string{mimeType}) {
		log.Print("detected MIME type: ", mimeType)
		return fmt.Errorf("invalid MIME type: %s", mimeType)
	}

	// Save the file to the server
	// Get the destination directory from the database
	var dir string
	if err := app.DB.QueryRow(
		"SELECT dir FROM upload_links WHERE token = ?",
		token,
	).Scan(&dir); err != nil {
		return fmt.Errorf("failed to get upload directory: %v", err)
	}
	dir = filepath.Join(app.Config.Upload.Path, dir)
	// Sanitize the filename
	filename := sanitizeFilename(handler.Filename, token)
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	// Save the file
	filepath := filepath.Join(dir, filename)
	// Check if the file already exists
	if _, err := os.Stat(filepath); err == nil {
		return fmt.Errorf("file already exists: %s", filepath)
	}
	outFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer outFile.Close()

	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file: %v", err)
	}
	if _, err := io.Copy(outFile, file); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}
	// Update the remaining usages in the database
	if _, err := app.DB.Exec(
		"UPDATE upload_links SET remaining_uses = remaining_uses - 1, last_used_at = ? WHERE token = ?",
		time.Now(),
		token,
	); err != nil {
		return fmt.Errorf("failed to update remaining usages: %v", err)
	}
	return nil
}

func (app *Application) validateExtension(ext string) bool {
	// Check if the file extension is allowed
	for _, allowedExt := range app.Config.Upload.AllowedExtensions {
		if ext == allowedExt {
			return true
		}
	}
	return false
}

func (app *Application) validateMimeType(mime []string) bool {
	// Check if the MIME type is allowed
	for _, allowedMime := range app.Config.Upload.AllowedMimeTypes {
		for _, m := range mime {
			if strings.HasPrefix(m, allowedMime) {
				return true
			}
		}
	}
	return false
}

func sanitizeFilename(filename, token string) string {
	// Remove any path components
	filename = filepath.Base(filename)
	// Remove any invalid characters
	filename = regexp.MustCompile(`[^a-zA-Z0-9\-_.]+`).ReplaceAllString(filename, "")
	// Prefix filename with token
	prefix := generateRandomString(16)
	filename = prefix + "_" + filename
	// Limit filename length
	if len(filename) > 255 {
		// Take the extension
		ext := filepath.Ext(filename)
		// Truncate the filename to 255 characters minus the extension length
		filename = filename[:255-len(ext)]
		// Add the extension back
		filename += ext
	}
	return filename
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
