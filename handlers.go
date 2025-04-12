package main

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"
)

// Route handlers
func (app *Application) homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/" {
		app.render404(w)
		return
	}
	// Check if user is already logged in
	if _, err := app.getSession(r); err == nil {
		// Session is valid, redirect to admin page
		http.Redirect(w, r, "/admin/home/", http.StatusFound)
		return
	}
	state := r.URL.Query().Get("state")
	data := struct {
		GoogleAuth bool
		StaticAuth bool
		Incorrect  bool
	}{
		GoogleAuth: app.Config.Auth.Type == AuthTypeGoogle,
		StaticAuth: app.Config.Auth.Type == AuthTypeStatic,
		Incorrect:  state != "",
	}
	// Render the home page
	app.renderTemplate(w, "home.html", data)
}

func (app *Application) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && app.Config.Auth.Type == AuthTypeGoogle {
		app.GoogleAuth.Redirect(w, r)
		return
	}
	if r.Method == http.MethodPost && app.Config.Auth.Type == AuthTypeStatic {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if ok := app.validateStaticAuth(r.FormValue("username"), r.FormValue("password")); !ok {
			log.Printf("Invalid login attempt: %s", r.FormValue("username"))
			http.Redirect(w, r, "/?state=1", http.StatusFound)
			return
		}
		app.createSession(w)
		http.Redirect(w, r, "/admin/home/", http.StatusFound)
	}
	// Invalid method/auth type combination
	log.Printf("Invalid login method or auth type (method: %s, auth type: %s)", r.Method, app.Config.Auth.Type)
	app.render404(w)
}

func (app *Application) googleAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if app.Config.Auth.Type != AuthTypeGoogle {
		app.render404(w)
		return
	}
	if err := app.GoogleAuth.Callback(w, r); err != nil {
		log.Printf("Google OAuth callback error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Set session cookie
	app.createSession(w)
	// Redirect to admin home
	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (app *Application) logoutHandler(w http.ResponseWriter, r *http.Request) {
	app.destroySession(w, r)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (app *Application) adminHomeHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch active links from the database
	var links []struct {
		RemainingUses int
		Token         string
		Dir           string
		ExpiresAt     time.Time
		CreatedAt     time.Time
		LastUsedAt    *time.Time
		Url           string
	}
	rows, err := app.DB.Query("SELECT remaining_uses, token, dir, expires_at, created_at, last_used_at FROM upload_links")
	if err != nil {
		log.Printf("Failed to fetch upload links: %v", err)
		http.Error(w, "Failed to fetch upload links", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var link struct {
			RemainingUses int
			Token         string
			Dir           string
			ExpiresAt     time.Time
			CreatedAt     time.Time
			LastUsedAt    *time.Time
			Url           string
		}
		if err := rows.Scan(&link.RemainingUses, &link.Token, &link.Dir, &link.ExpiresAt, &link.CreatedAt, &link.LastUsedAt); err != nil {
			log.Printf("Failed to scan upload link: %v", err)
			http.Error(w, "Failed to fetch upload links", http.StatusInternalServerError)
			return
		}
		link.Url = app.Config.BaseURL + "/upload/" + link.Token + "/"
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating over upload links: %v", err)
		http.Error(w, "Failed to fetch upload links", http.StatusInternalServerError)
		return
	}
	// Render the admin home page with the links
	data := struct {
		Links []struct {
			RemainingUses int
			Token         string
			Dir           string
			ExpiresAt     time.Time
			CreatedAt     time.Time
			LastUsedAt    *time.Time
			Url           string
		}
	}{
		Links: links,
	}
	app.renderTemplate(w, "admin_home.html", data)
}

func (app *Application) deactivateLinkHandler(w http.ResponseWriter, r *http.Request) {
	// Validate the request
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	// Get the token from the form
	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}
	// Deactivate the link in the database
	if err := app.deactivateUploadLink(token); err != nil {
		log.Printf("Failed to deactivate upload link: %v", err)
		http.Error(w, "Failed to deactivate upload link", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (app *Application) createLinkHandler(w http.ResponseWriter, r *http.Request) {
	// Validate the request
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
	//
	if err := app.createUploadLink(directory, expiresAt, remainingUses); err != nil {
		log.Printf("Failed to create upload link: %v", err)
		http.Error(w, "Failed to create upload link", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/home/", http.StatusFound)
}

func (app *Application) getUploadHandler(w http.ResponseWriter, r *http.Request) {
	token, err := app.validateUploadToken(r)
	if err != nil {
		log.Printf("Invalid upload token: %v", err)
		app.render404(w)
		return
	}
	data := struct {
		Token string
	}{
		Token: token,
	}
	app.renderTemplate(w, "upload.html", data)
}
func (app *Application) postUploadHandler(w http.ResponseWriter, r *http.Request) {
	token, err := app.validateUploadToken(r)
	if err != nil {
		log.Printf("Invalid upload token: %v", err)
		app.render404(w)
		return
	}

	// Process file upload
	if err := app.upload(r, token); err != nil {
		log.Printf("File upload error: %v", err)
		http.Redirect(w, r, "/upload/error/", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/upload/success/", http.StatusFound)
}

// Modify renderTemplate function to use templates from disk
func (app *Application) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	if app.Config.IsDevelopment {
		// Reload templates in development mode
		templates, err := template.ParseGlob(filepath.Join(app.Config.TemplatePath, "*.html"))
		if err != nil {
			http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		app.Templates = templates
	}

	// Execute the template
	if err := app.Templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Template rendering error: "+err.Error(), http.StatusInternalServerError)
	}
}

func (app *Application) render404(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	app.renderTemplate(w, "404.html", nil)
}
