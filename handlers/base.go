package handlers

import (
	"apicalls/auth"
	"apicalls/logger"
	"apicalls/models"
	"apicalls/storage"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
)

// AppVersion and AppBuildTime are set by main at startup.
var (
	AppVersion   = "dev"
	AppBuildTime = "unknown"
)

// Base holds shared dependencies for all handlers.
type Base struct {
	DB      *storage.DB
	Auth    *auth.Manager
	TmplDir string
}

// PageData is the common data passed to all HTML templates.
type PageData struct {
	User         *models.User
	ClientIP     string
	Flash        string
	ActiveMenu   string
	Environments []*models.Environment // loaded once, available globally
	AppVersion   string
	AppBuildTime string
	Data         interface{}
}

// render executes a named template set: layout.html + the given page template.
func (b *Base) render(w http.ResponseWriter, r *http.Request, page string, data *PageData) {
	if data.User == nil {
		data.User = b.Auth.CurrentUser(r)
	}
	data.ClientIP = auth.ClientIP(r)
	data.AppVersion = AppVersion
	data.AppBuildTime = AppBuildTime

	// Load environments for the nav bar env indicator
	if data.Environments == nil {
		data.Environments, _ = b.DB.ListEnvironments()
	}

	files := []string{
		filepath.Join(b.TmplDir, "layout.html"),
		filepath.Join(b.TmplDir, page),
	}
	tmpl, err := template.New("layout").Funcs(templateFuncs()).ParseFiles(files...)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}

// log is a convenience wrapper.
func (b *Base) log(r *http.Request, module, desc string) {
	user := b.Auth.CurrentUser(r)
	username := "anonymous"
	if user != nil {
		username = user.Username
	}
	logger.Log(username, module, desc)
}

// techLog writes a structured entry to the technical log.
// level should be logger.LevelInfo / LevelWarn / LevelError.
func (b *Base) techLog(r *http.Request, level, module, message string) {
	user := b.Auth.CurrentUser(r)
	username := "anonymous"
	if user != nil {
		username = user.Username
	}
	logger.Tech(level, username, module, message)
}

// flashSession stores a one-time flash message in the cookie session.
// We keep it simple: redirect with a query param.
func flashRedirect(w http.ResponseWriter, r *http.Request, url, msg string) {
	if msg != "" {
		sep := "?"
		if strings.Contains(url, "?") {
			sep = "&"
		}
		url += sep + "flash=" + template.URLQueryEscaper(msg)
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"maskPassword": func(s string) string {
			if s == "" {
				return ""
			}
			return "••••••••"
		},
		"roleLabel": auth.RoleLabel,
		"add":       func(a, b int) int { return a + b },
		"deref": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		},
	}
}
