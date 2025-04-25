package handlers

import (
	"fmt"
	"github.com/frodejac/globster/internal/auth"
	"github.com/frodejac/globster/internal/config"
	"github.com/frodejac/globster/internal/database/links"
	"github.com/frodejac/globster/internal/downloads"
	"github.com/frodejac/globster/internal/files"
	"github.com/frodejac/globster/internal/uploads"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type AdminHandler struct {
	BaseHandler
	baseUrl   string
	linkStore *links.Store
	uploads   *uploads.UploadService
	downloads *downloads.DownloadService
	files     *files.FileService
}

func NewAdminHandler(authType config.AuthType, baseUrl string, sessions *auth.SessionService, templates *template.Template, linkStore *links.Store, uploads *uploads.UploadService, downloads *downloads.DownloadService, files *files.FileService) *AdminHandler {
	return &AdminHandler{
		BaseHandler: BaseHandler{
			authType:  authType,
			sessions:  sessions,
			templates: templates,
		},
		baseUrl:   baseUrl,
		linkStore: linkStore,
		uploads:   uploads,
		downloads: downloads,
		files:     files,
	}
}

type AdminData struct {
	UploadLinks   []links.UploadLink
	Directories   []files.Directory
	Directory     *files.Directory
	DownloadLinks []links.DownloadLink
}

func (h *AdminHandler) HandleHome(w http.ResponseWriter, r *http.Request) {
	activeLinks, err := h.linkStore.ListActiveUploadLinks()
	if err != nil {
		slog.Error("Failed to fetch active links", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for i := range activeLinks {
		activeLinks[i].Url = h.baseUrl + activeLinks[i].Url
	}

	h.renderTemplate(w, "admin_home.html", AdminData{UploadLinks: activeLinks})
}

func (h *AdminHandler) HandleCreateLink(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Get the parameters from the form
	directory := r.FormValue("directory")
	if directory == "" {
		http.Error(w, "Missing directory", http.StatusBadRequest)
		return
	}
	expiresInStr := r.FormValue("expiresIn")
	if expiresInStr == "" {
		http.Error(w, "Missing expiration", http.StatusBadRequest)
		return
	}
	var expiresAt time.Time
	expiresIn, err := time.ParseDuration(expiresInStr)
	if err != nil {
		http.Error(w, "Invalid expiration duration", http.StatusBadRequest)
		return
	}
	expiresAt = time.Now().Add(expiresIn)

	uses := r.FormValue("uses")
	if uses == "" {
		http.Error(w, "Missing remaining uses", http.StatusBadRequest)
		return
	}
	remainingUses, err := strconv.Atoi(uses)
	if err != nil {
		http.Error(w, "Invalid remaining uses", http.StatusBadRequest)
		return
	}
	if err := h.uploads.CreateLink(directory, expiresAt, remainingUses); err != nil {
		slog.Error("Failed to create upload link", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (h *AdminHandler) HandleDeactivateLink(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Get the token from the form
	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Deactivate the link in the database
	if err := h.uploads.DeactivateLink(token); err != nil {
		slog.Error("Failed to deactivate upload link", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (h *AdminHandler) HandleListDirectories(w http.ResponseWriter, r *http.Request) {
	directories, err := h.files.ListDirectories()
	if err != nil {
		slog.Error("Failed to fetch directories", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	h.renderTemplate(w, "admin_directories.html", AdminData{Directories: directories})
}

func (h *AdminHandler) HandleListDirectory(w http.ResponseWriter, r *http.Request) {
	dirName := r.PathValue("directory")
	if dirName == "" {
		http.Error(w, "Missing directory", http.StatusBadRequest)
		return
	}
	directory, err := h.files.ListFiles(dirName)
	if err != nil {
		slog.Error("Failed to fetch files", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if directory == nil {
		slog.Error("Directory not found", "directory", dirName)
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	}
	downloadLinks, err := h.linkStore.ListActiveDownloadLinks()
	if err != nil {
		slog.Error("Failed to fetch download links", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	for i := range downloadLinks {
		downloadLinks[i].Url = h.baseUrl + downloadLinks[i].Url
	}
	h.renderTemplate(w, "admin_directory.html", AdminData{Directory: directory, DownloadLinks: downloadLinks})
}

func (h *AdminHandler) HandleDownloadFile(w http.ResponseWriter, r *http.Request) {
	dirName := r.PathValue("directory")
	fileName := r.PathValue("filename")
	if dirName == "" || fileName == "" {
		http.Error(w, "Missing directory or filename", http.StatusBadRequest)
		return
	}
	filePath, fileInfo, err := h.files.GetFilePath(dirName, fileName)
	if err != nil {
		slog.Error("Failed to get file path", "error", err)
		h.render404(w)
		return
	}
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("Failed to open file", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", h.files.DisplayName(fileInfo.Name())))
	http.ServeContent(w, r, filePath, fileInfo.ModTime(), file)
}

func (h *AdminHandler) HandleShareDirectory(w http.ResponseWriter, r *http.Request) {
	dirName := r.PathValue("directory")
	if dirName == "" {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	expiresInStr := r.FormValue("expiresIn")
	if expiresInStr == "" {
		http.Error(w, "Missing expiration", http.StatusBadRequest)
		return
	}
	var expiresAt time.Time
	expiresIn, err := time.ParseDuration(expiresInStr)
	if err != nil {
		http.Error(w, "Invalid expiration duration", http.StatusBadRequest)
		return
	}
	expiresAt = time.Now().Add(expiresIn)

	uses := r.FormValue("uses")
	if uses == "" {
		http.Error(w, "Missing remaining uses", http.StatusBadRequest)
		return
	}
	remainingUses, err := strconv.Atoi(uses)
	if err != nil {
		http.Error(w, "Invalid remaining uses", http.StatusBadRequest)
		return
	}

	if err := h.downloads.CreateLink(dirName, expiresAt, remainingUses); err != nil {
		slog.Error("Failed to create download link", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/files/%s/", dirName), http.StatusFound)
}

func (h *AdminHandler) HandleUnshareDirectory(w http.ResponseWriter, r *http.Request) {
	dirName := r.PathValue("directory")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Get the token from the form
	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Deactivate the link in the database
	if err := h.downloads.DeactivateLink(token); err != nil {
		slog.Error("Failed to deactivate download link", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/files/%s/", dirName), http.StatusFound)
}

func (h *AdminHandler) HandlePostUpload(w http.ResponseWriter, r *http.Request) {
	directory := r.PathValue("directory")
	if err := h.uploads.AdminUpload(r, directory); err != nil {
		slog.Error("Upload error", "error", err)
		http.Redirect(w, r, "/upload/error/", http.StatusFound)
		return
	}
	// Remove the /upload suffix from the URL
	redirectUrl := strings.TrimSuffix(r.URL.Path, "upload/")
	http.Redirect(w, r, redirectUrl, http.StatusFound)
}
