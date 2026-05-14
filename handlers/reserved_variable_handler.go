package handlers

import (
	"apicalls/logger"
	"apicalls/models"
	"encoding/json"
	"fmt"
	"net/http"
)

// ReservedVariableHandler serves the reserved variables module.
type ReservedVariableHandler struct {
	Base
}

type reservedVarsPageData struct {
	ReservedVars []*models.ReservedVariable
	CanEdit      bool // admin only
}

// Page renders the reserved variables list.
func (h *ReservedVariableHandler) Page(w http.ResponseWriter, r *http.Request) {
	h.log(r, "reserved_variables", "Accessed reserved variables module")
	user := h.Auth.CurrentUser(r)

	vars, err := h.DB.ListReservedVariables()
	if err != nil {
		h.techLog(r, logger.LevelError, "reserved_variables",
			fmt.Sprintf("Page DB error | op=ListReservedVariables error_type=DB_ERROR error=%q", err.Error()))
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	flash := r.URL.Query().Get("flash")
	h.render(w, r, "reserved_variables.html", &PageData{
		User:       user,
		ActiveMenu: "reserved_variables",
		Flash:      flash,
		Data: &reservedVarsPageData{
			ReservedVars: vars,
			CanEdit:      user.HasRole(models.RoleAdmin),
		},
	})
}

// UpdateDescription updates the description of a reserved variable (admin only).
func (h *ReservedVariableHandler) UpdateDescription(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")

	if !models.IsReservedVarName(name) {
		flashRedirect(w, r, "/reserved-variables", "Unknown reserved variable: "+name)
		return
	}

	if err := h.DB.UpdateReservedVariableDescription(name, description); err != nil {
		h.techLog(r, logger.LevelError, "reserved_variables",
			fmt.Sprintf("UpdateDescription DB error | name=%s error_type=DB_ERROR error=%q", name, err.Error()))
		flashRedirect(w, r, "/reserved-variables", "Error: "+err.Error())
		return
	}

	h.log(r, "reserved_variables", "Updated description for reserved variable: "+name)
	flashRedirect(w, r, "/reserved-variables", "Description updated")
}

// GetReservedVariablesJSON returns the current resolved values of all reserved
// variables for use by the execution UI.
func (h *ReservedVariableHandler) GetReservedVariablesJSON(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	serviceName := r.URL.Query().Get("service_name")

	values := resolveReservedVarMap(user.Username, serviceName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(values)
}
