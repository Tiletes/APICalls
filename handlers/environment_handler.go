package handlers

import (
	"apicalls/logger"
	"apicalls/models"
	"net/http"
	"strconv"
)

// EnvironmentHandler manages environments.
type EnvironmentHandler struct {
	Base
}

func (h *EnvironmentHandler) List(w http.ResponseWriter, r *http.Request) {
	h.log(r, "environments", "Accessed environments module")
	user := h.Auth.CurrentUser(r)
	envs, err := h.DB.ListEnvironments()
	if err != nil {
		h.techLog(r, logger.LevelError, "environments",
			"List DB error | op=ListEnvironments error_type=DB_ERROR error="+err.Error())
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	flash := r.URL.Query().Get("flash")
	h.render(w, r, "environments.html", &PageData{
		User:       user,
		ActiveMenu: "environments",
		Flash:      flash,
		Data:       envs,
	})
}

func (h *EnvironmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	priority, _ := strconv.Atoi(r.FormValue("priority"))
	env := &models.Environment{
		Name:     r.FormValue("name"),
		Color:    r.FormValue("color"),
		Priority: priority,
	}
	if err := h.DB.CreateEnvironment(env); err != nil {
		h.techLog(r, logger.LevelError, "environments",
			"Create DB error | op=CreateEnvironment name="+env.Name+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/environments", "Error: "+err.Error())
		return
	}
	h.log(r, "environments", "Created environment: "+env.Name)
	flashRedirect(w, r, "/environments", "Environment created")
}

func (h *EnvironmentHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	priority, _ := strconv.Atoi(r.FormValue("priority"))
	env := &models.Environment{
		ID:       id,
		Name:     r.FormValue("name"),
		Color:    r.FormValue("color"),
		Priority: priority,
	}
	if err := h.DB.UpdateEnvironment(env); err != nil {
		h.techLog(r, logger.LevelError, "environments",
			"Update DB error | op=UpdateEnvironment id="+strconv.FormatInt(env.ID, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/environments", "Error: "+err.Error())
		return
	}
	h.log(r, "environments", "Updated environment: "+env.Name)
	flashRedirect(w, r, "/environments", "Environment updated")
}

func (h *EnvironmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
	if err := h.DB.DeleteEnvironment(id); err != nil {
		h.techLog(r, logger.LevelError, "environments",
			"Delete DB error | op=DeleteEnvironment id="+strconv.FormatInt(id, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/environments", "Error: "+err.Error())
		return
	}
	h.log(r, "environments", "Deleted environment ID: "+strconv.FormatInt(id, 10))
	flashRedirect(w, r, "/environments", "Environment deleted")
}
