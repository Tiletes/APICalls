package handlers

import (
	"apicalls/logger"
	"apicalls/models"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// TechnologyHandler manages technologies.
type TechnologyHandler struct {
	Base
}

func (h *TechnologyHandler) List(w http.ResponseWriter, r *http.Request) {
	h.log(r, "technologies", "Accessed technologies module")
	user := h.Auth.CurrentUser(r)
	techs, err := h.DB.ListTechnologies()
	if err != nil {
		h.techLog(r, logger.LevelError, "technologies",
			"List DB error | op=ListTechnologies error_type=DB_ERROR error="+err.Error())
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	flash := r.URL.Query().Get("flash")
	h.render(w, r, "technologies.html", &PageData{
		User:       user,
		ActiveMenu: "technologies",
		Flash:      flash,
		Data: map[string]interface{}{
			"Technologies": techs,
			"CanEdit":      user.HasRole(models.RoleAdmin, models.RoleStandard),
		},
	})
}

func (h *TechnologyHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	t := &models.Technology{
		Name:   r.FormValue("name"),
		Method: r.FormValue("method"),
		URL:    r.FormValue("url"),
		Body:   r.FormValue("body"),
	}
	json.Unmarshal([]byte(r.FormValue("headers_json")), &t.Headers)
	json.Unmarshal([]byte(r.FormValue("custom_values_json")), &t.CustomValues)
	if err := h.DB.CreateTechnology(t); err != nil {
		h.techLog(r, logger.LevelError, "technologies",
			"Create DB error | op=CreateTechnology name="+t.Name+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/technologies", "Error: "+err.Error())
		return
	}
	h.log(r, "technologies", "Created technology: "+t.Name)
	flashRedirect(w, r, "/technologies", "Technology created")
}

func (h *TechnologyHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	t := &models.Technology{
		ID:     id,
		Name:   r.FormValue("name"),
		Method: r.FormValue("method"),
		URL:    r.FormValue("url"),
		Body:   r.FormValue("body"),
	}
	json.Unmarshal([]byte(r.FormValue("headers_json")), &t.Headers)
	json.Unmarshal([]byte(r.FormValue("custom_values_json")), &t.CustomValues)
	if err := h.DB.UpdateTechnology(t); err != nil {
		h.techLog(r, logger.LevelError, "technologies",
			"Update DB error | op=UpdateTechnology id="+strconv.FormatInt(t.ID, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/technologies", "Error: "+err.Error())
		return
	}
	h.log(r, "technologies", "Updated technology: "+t.Name)
	flashRedirect(w, r, "/technologies", "Technology updated")
}

func (h *TechnologyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err := h.DB.DeleteTechnology(id); err != nil {
		h.techLog(r, logger.LevelError, "technologies",
			"Delete DB error | op=DeleteTechnology id="+strconv.FormatInt(id, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/technologies", "Error: "+err.Error())
		return
	}
	h.log(r, "technologies", "Deleted technology ID: "+strconv.FormatInt(id, 10))
	flashRedirect(w, r, "/technologies", "Technology deleted")
}

// GetTechnologyJSON returns a technology as JSON (for auto-fill in template editor).
func (h *TechnologyHandler) GetTechnologyJSON(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.ParseInt(idStr, 10, 64)
	t, err := h.DB.GetTechnologyByID(id)
	if err != nil || t == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}
