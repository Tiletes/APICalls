package handlers

import (
	"apicalls/logger"
	"apicalls/models"
	"net/http"
	"strconv"
	"strings"
)

// VariableHandler manages substitution variables.
type VariableHandler struct {
	Base
}

type variablesPageData struct {
	Variables    []*models.Variable
	Environments []*models.Environment
	Search       string
	CanEdit      bool
	CanUnmask    bool
}

func (h *VariableHandler) List(w http.ResponseWriter, r *http.Request) {
	h.log(r, "variables", "Accessed variables module")
	user := h.Auth.CurrentUser(r)

	if user.HasRole(models.RoleGuest) {
		envs, _ := h.DB.ListEnvironments()
		h.render(w, r, "variables.html", &PageData{
			User:       user,
			ActiveMenu: "variables",
			Data: &variablesPageData{
				Environments: envs,
			},
		})
		return
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	vars, err := h.DB.ListVariables()
	if err != nil {
		h.techLog(r, logger.LevelError, "variables",
			"List DB error | op=ListVariables error_type=DB_ERROR error="+err.Error())
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	if search != "" {
		filtered := vars[:0]
		lower := strings.ToLower(search)
		for _, v := range vars {
			if strings.Contains(strings.ToLower(v.Name), lower) {
				filtered = append(filtered, v)
			}
		}
		vars = filtered
	}

	envs, _ := h.DB.ListEnvironments()
	flash := r.URL.Query().Get("flash")

	h.render(w, r, "variables.html", &PageData{
		User:       user,
		ActiveMenu: "variables",
		Flash:      flash,
		Data: &variablesPageData{
			Variables:    vars,
			Environments: envs,
			Search:       search,
			CanEdit:      user.HasRole(models.RoleAdmin, models.RoleStandard),
			CanUnmask:    user.HasRole(models.RoleAdmin),
		},
	})
}

func (h *VariableHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	envs, _ := h.DB.ListEnvironments()
	v := &models.Variable{
		Name:       r.FormValue("name"),
		IsPassword: r.FormValue("is_password") == "1",
		Values:     make(map[string]string),
	}
	if models.IsReservedVarName(v.Name) {
		flashRedirect(w, r, "/variables", "Error: '"+v.Name+"' is a reserved keyword and cannot be used as a variable name")
		return
	}
	for _, env := range envs {
		v.Values[env.Name] = r.FormValue("val_" + env.Name)
	}
	if err := h.DB.CreateVariable(v); err != nil {
		h.techLog(r, logger.LevelError, "variables",
			"Create DB error | op=CreateVariable name="+v.Name+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/variables", "Error: "+err.Error())
		return
	}
	h.log(r, "variables", "Created variable: "+v.Name)
	flashRedirect(w, r, "/variables", "Variable created")
}

func (h *VariableHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	envs, _ := h.DB.ListEnvironments()
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	v := &models.Variable{
		ID:         id,
		Name:       r.FormValue("name"),
		IsPassword: r.FormValue("is_password") == "1",
		Values:     make(map[string]string),
	}
	if models.IsReservedVarName(v.Name) {
		flashRedirect(w, r, "/variables", "Error: '"+v.Name+"' is a reserved keyword and cannot be used as a variable name")
		return
	}
	for _, env := range envs {
		v.Values[env.Name] = r.FormValue("val_" + env.Name)
	}
	if err := h.DB.UpdateVariable(v); err != nil {
		h.techLog(r, logger.LevelError, "variables",
			"Update DB error | op=UpdateVariable id="+strconv.FormatInt(v.ID, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/variables", "Error: "+err.Error())
		return
	}
	h.log(r, "variables", "Updated variable: "+v.Name)
	flashRedirect(w, r, "/variables", "Variable updated")
}

func (h *VariableHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
	if err := h.DB.DeleteVariable(id); err != nil {
		h.techLog(r, logger.LevelError, "variables",
			"Delete DB error | op=DeleteVariable id="+strconv.FormatInt(id, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/variables", "Error: "+err.Error())
		return
	}
	h.log(r, "variables", "Deleted variable ID: "+strconv.FormatInt(id, 10))
	flashRedirect(w, r, "/variables", "Variable deleted")
}
