package handlers

import (
	"fmt"
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/config"
	"github.com/frodejac/globster/internal/downloads"
	"github.com/frodejac/globster/internal/files"
	"html/template"
	"log/slog"
	"net/http"
	"os"
)

type DownloadHandler struct {
	BaseHandler
	downloads *downloads.DownloadService
	files     *files.FileService
}

type DownloadData struct {
	Directory *files.Directory
	Token     string
}

func NewDownloadHandler(authType config.AuthType, sessions *auth.SessionService, templates *template.Template, downloads *downloads.DownloadService, files *files.FileService) *DownloadHandler {
	return &DownloadHandler{
		BaseHandler: BaseHandler{
			authType:  authType,
			sessions:  sessions,
			templates: templates,
		},
		downloads: downloads,
		files:     files,
	}
}

func (h *DownloadHandler) HandleGetDirectory(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	link, err := h.downloads.ValidateToken(token)
	if err != nil {
		h.render404(w)
		return
	}
	directory, err := h.files.ListFiles(link.Dir)
	if err != nil {
		h.render404(w)
		return
	}
	h.renderTemplate(w, "download.html", DownloadData{Token: token, Directory: directory})
}

func (h *DownloadHandler) HandleGetFile(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	fileName := r.PathValue("file")
	link, err := h.downloads.ValidateToken(token)
	if err != nil {
		h.render404(w)
		return
	}
	filePath, fileInfo, err := h.files.GetFilePath(link.Dir, fileName)
	if err != nil {
		slog.Error("Failed to get file path", "error", err)
		h.render404(w)
		return
	}
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("Failed to open file", "error", err)
		h.render404(w)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", h.files.DisplayName(fileInfo.Name())))
	http.ServeContent(w, r, filePath, fileInfo.ModTime(), file)
}
