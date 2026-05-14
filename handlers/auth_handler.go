package handlers

import (
	"apicalls/logger"
	"net/http"
)

// AuthHandler handles login and logout.
type AuthHandler struct {
	Base
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if h.Auth.CurrentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	flash := r.URL.Query().Get("flash")
	h.render(w, r, "login.html", &PageData{Flash: flash, ActiveMenu: "login"})
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.Auth.Login(w, r, username, password)
	if err != nil || user == nil {
		flashRedirect(w, r, "/login", "Invalid username or password")
		return
	}
	logger.Log(user.Username, "auth", "User logged in")
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if user != nil {
		logger.Log(user.Username, "auth", "User logged out")
	}
	h.Auth.Logout(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *AuthHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	h.render(w, r, "dashboard.html", &PageData{User: user, ActiveMenu: "dashboard"})
}
