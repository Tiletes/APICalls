package handlers

import (
	"apicalls/logger"
	"apicalls/models"
	"apicalls/wsdl"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// TemplateHandler manages request templates.
type TemplateHandler struct {
	Base
}

type templatePageData struct {
	Templates    []*models.Template
	Technologies []*models.Technology
	CanEdit      bool
}

func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	h.log(r, "templates", "Accessed templates module")
	user := h.Auth.CurrentUser(r)
	tmplList, err := h.DB.ListTemplates()
	if err != nil {
		h.techLog(r, logger.LevelError, "templates",
			"List DB error | op=ListTemplates error_type=DB_ERROR error="+err.Error())
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	techs, _ := h.DB.ListTechnologies()
	flash := r.URL.Query().Get("flash")
	h.render(w, r, "templates.html", &PageData{
		User:       user,
		ActiveMenu: "templates",
		Flash:      flash,
		Data: &templatePageData{
			Templates:    tmplList,
			Technologies: techs,
			CanEdit:      user.HasRole(models.RoleAdmin, models.RoleStandard),
		},
	})
}

func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	// ParseMultipartForm handles both multipart/form-data (WSDL import via fetch)
	// and application/x-www-form-urlencoded (regular HTML form submit).
	// It internally calls ParseForm for non-multipart bodies.
	r.ParseMultipartForm(10 << 20) //nolint — error is intentionally ignored
	t := &models.Template{
		Name:                r.FormValue("name"),
		ServiceName:         r.FormValue("service_name"),
		Path:                r.FormValue("path"),
		Method:              r.FormValue("method"),
		URL:                 r.FormValue("url"),
		Body:                r.FormValue("body"),
		RestrictedExecution: r.FormValue("restricted_execution") == "1",
	}
	json.Unmarshal([]byte(r.FormValue("headers_json")), &t.Headers)
	json.Unmarshal([]byte(r.FormValue("custom_values_json")), &t.CustomValues)
	techIDStr := r.FormValue("technology_id")
	if techIDStr != "" && techIDStr != "0" {
		techID, _ := strconv.ParseInt(techIDStr, 10, 64)
		t.TechnologyID = &techID
	}
	if err := h.DB.CreateTemplate(t); err != nil {
		h.techLog(r, logger.LevelError, "templates",
			"Create DB error | op=CreateTemplate name="+t.Name+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/templates", "Error: "+err.Error())
		return
	}
	h.log(r, "templates", "Created template: "+t.Name)
	flashRedirect(w, r, "/templates", "Template created")
}

func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	t := &models.Template{
		ID:                  id,
		Name:                r.FormValue("name"),
		ServiceName:         r.FormValue("service_name"),
		Path:                r.FormValue("path"),
		Method:              r.FormValue("method"),
		URL:                 r.FormValue("url"),
		Body:                r.FormValue("body"),
		RestrictedExecution: r.FormValue("restricted_execution") == "1",
	}
	json.Unmarshal([]byte(r.FormValue("headers_json")), &t.Headers)
	json.Unmarshal([]byte(r.FormValue("custom_values_json")), &t.CustomValues)
	techIDStr := r.FormValue("technology_id")
	if techIDStr != "" && techIDStr != "0" {
		techID, _ := strconv.ParseInt(techIDStr, 10, 64)
		t.TechnologyID = &techID
	}
	if err := h.DB.UpdateTemplate(t); err != nil {
		h.techLog(r, logger.LevelError, "templates",
			"Update DB error | op=UpdateTemplate id="+strconv.FormatInt(t.ID, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/templates", "Error: "+err.Error())
		return
	}
	h.log(r, "templates", "Updated template: "+t.Name)
	flashRedirect(w, r, "/templates", "Template updated")
}

func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
	if err := h.DB.DeleteTemplate(id); err != nil {
		h.techLog(r, logger.LevelError, "templates",
			"Delete DB error | op=DeleteTemplate id="+strconv.FormatInt(id, 10)+" error_type=DB_ERROR error="+err.Error())
		flashRedirect(w, r, "/templates", "Error: "+err.Error())
		return
	}
	h.log(r, "templates", "Deleted template ID: "+strconv.FormatInt(id, 10))
	flashRedirect(w, r, "/templates", "Template deleted")
}

// SaveCopy creates a new template as a copy of an existing one, with a new name.
func (h *TemplateHandler) SaveCopy(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	srcID, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	newName := strings.TrimSpace(r.FormValue("name"))
	if newName == "" {
		flashRedirect(w, r, "/templates", "Error: name is required")
		return
	}
	src, err := h.DB.GetTemplateByID(srcID)
	if err != nil || src == nil {
		flashRedirect(w, r, "/templates", "Error: source template not found")
		return
	}
	copy := &models.Template{
		Name:                newName,
		ServiceName:         src.ServiceName,
		Path:                src.Path,
		Method:              src.Method,
		URL:                 src.URL,
		Body:                src.Body,
		Headers:             src.Headers,
		CustomValues:        src.CustomValues,
		RestrictedExecution: src.RestrictedExecution,
		TechnologyID:        src.TechnologyID,
	}
	if err := h.DB.CreateTemplate(copy); err != nil {
		h.techLog(r, logger.LevelError, "templates",
			fmt.Sprintf("SaveCopy DB error | op=CreateTemplate name=%q error_type=DB_ERROR error=%q", newName, err.Error()))
		flashRedirect(w, r, "/templates", "Error: "+err.Error())
		return
	}
	h.log(r, "templates", fmt.Sprintf("Copied template %d to '%s'", srcID, newName))
	flashRedirect(w, r, "/templates", "Template copied as '"+newName+"'")
}

// UniqueName checks whether a template name is already in use.
// Returns JSON: {"unique": true|false}
func (h *TemplateHandler) UniqueName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	excludeID, _ := strconv.ParseInt(r.URL.Query().Get("exclude_id"), 10, 64)
	templates, err := h.DB.ListTemplates()
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	unique := true
	for _, t := range templates {
		if strings.EqualFold(t.Name, name) && t.ID != excludeID {
			unique = false
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"unique": unique})
}

// ParseWSDL fetches/parses a WSDL and returns the detected operations as JSON.
// Accepts JSON body: {"url": "..."} or multipart with file upload.
func (h *TemplateHandler) ParseWSDL(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	writeErr := func(msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": msg})
	}

	contentType := r.Header.Get("Content-Type")

	var ops []wsdl.ParsedOperation
	var err error

	if strings.HasPrefix(contentType, "application/json") {
		// Fetch from URL
		var body struct {
			URL string `json:"url"`
		}
		if err = json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
			writeErr("provide a WSDL URL in the request body as {\"url\": \"...\"}")
			return
		}
		h.log(r, "templates", "WSDL parse from URL: "+body.URL)
		ops, err = wsdl.FetchFromURL(body.URL)
	} else {
		// File upload
		if err = r.ParseMultipartForm(5 << 20); err != nil {
			writeErr("could not parse multipart form")
			return
		}
		file, _, ferr := r.FormFile("wsdl_file")
		if ferr != nil {
			writeErr("provide a WSDL file in the 'wsdl_file' field")
			return
		}
		defer file.Close()
		var data []byte
		buf := make([]byte, 5<<20)
		n, _ := file.Read(buf)
		data = buf[:n]
		h.log(r, "templates", "WSDL parse from uploaded file")
		ops, err = wsdl.Parse(data)
	}

	if err != nil {
		h.techLog(r, logger.LevelWarn, "templates",
			fmt.Sprintf("ParseWSDL error | error_type=REQUEST_ERROR error=%q", err.Error()))
		writeErr(err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ops)
}
