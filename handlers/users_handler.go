package handlers

import (
	"apicalls/logger"
	"apicalls/models"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

// UsersHandler manages user accounts (admin only).
type UsersHandler struct {
	Base
}

var allRoles = []string{
	models.RoleAdmin,
	models.RoleStandard,
	models.RoleRestricted,
	models.RoleGuest,
}

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	h.log(r, "users", "Accessed users module")
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	users, err := h.DB.ListUsers()
	if err != nil {
		h.techLog(r, logger.LevelError, "users",
			"List DB error | op=ListUsers error_type=DB_ERROR error="+err.Error())
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	flash := r.URL.Query().Get("flash")
	h.render(w, r, "users.html", &PageData{
		User:       user,
		ActiveMenu: "users",
		Flash:      flash,
		Data: map[string]interface{}{
			"Users": users,
			"Roles": allRoles,
		},
	})
}

func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	password := r.FormValue("password")
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		flashRedirect(w, r, "/users", "Error hashing password")
		return
	}
	role := r.FormValue("role")
	if role == "" {
		role = models.RoleStandard
	}
	newUser := &models.User{
		Username: r.FormValue("username"),
		Password: string(hash),
		Role:     role,
	}
	if err := h.DB.CreateUser(newUser); err != nil {
		h.techLog(r, logger.LevelError, "users",
			"Create DB error | op=CreateUser username="+newUser.Username+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/users", "Error: "+err.Error())
		return
	}
	h.log(r, "users", "Created user: "+newUser.Username)
	flashRedirect(w, r, "/users", "User created")
}

func (h *UsersHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	role := r.FormValue("role")
	if role == "" {
		role = models.RoleStandard
	}
	updUser := &models.User{
		ID:       id,
		Username: r.FormValue("username"),
		Role:     role,
	}
	// Only update password if provided
	password := r.FormValue("password")
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			flashRedirect(w, r, "/users", "Error hashing password")
			return
		}
		updUser.Password = string(hash)
	} else {
		// Keep existing password
		existing, err := h.DB.ListUsers()
		if err != nil {
			h.techLog(r, logger.LevelError, "users",
				"Update DB error (fetch existing) | op=ListUsers error_type=DB_ERROR error="+err.Error())
			flashRedirect(w, r, "/users", "DB error")
			return
		}
		for _, u := range existing {
			if u.ID == id {
				updUser.Password = u.Password
				break
			}
		}
	}
	if err := h.DB.UpdateUser(updUser); err != nil {
		h.techLog(r, logger.LevelError, "users",
			"Update DB error | op=UpdateUser username="+updUser.Username+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/users", "Error: "+err.Error())
		return
	}
	h.log(r, "users", "Updated user: "+updUser.Username)
	flashRedirect(w, r, "/users", "User updated")
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	// Prevent self-deletion
	currentUser := h.Auth.CurrentUser(r)
	if currentUser.ID == id {
		flashRedirect(w, r, "/users", "Cannot delete your own account")
		return
	}
	if err := h.DB.DeleteUser(id); err != nil {
		h.techLog(r, logger.LevelError, "users",
			"Delete DB error | op=DeleteUser id="+strconv.FormatInt(id, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/users", "Error: "+err.Error())
		return
	}
	h.log(r, "users", "Deleted user ID: "+strconv.FormatInt(id, 10))
	flashRedirect(w, r, "/users", "User deleted")
}
