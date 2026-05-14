package handlers

import (
	"apicalls/logger"
	"apicalls/models"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// ExecutionHandler handles template execution.
type ExecutionHandler struct {
	Base
}

type executionPageData struct {
	Environments []*models.Environment
	Templates    []*models.Template
	Notes        []*models.Note
	SelectedEnv  *models.Environment
	SelectedTmpl *models.Template
	Result       *execResult
	CanRun       bool
	CanAddNote   bool
	CanEdit      bool
}

type execResult struct {
	StatusCode int
	Status     string
	Headers    map[string][]string
	Body       string
	Duration   time.Duration
	Error      string
}

func (h *ExecutionHandler) Page(w http.ResponseWriter, r *http.Request) {
	h.log(r, "execution", "Accessed execution module")
	user := h.Auth.CurrentUser(r)

	envs, err := h.DB.ListEnvironments()
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Page DB error | op=ListEnvironments error_type=DB_ERROR error=%q", err.Error()))
	}
	tmplList, err := h.DB.ListTemplates()
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Page DB error | op=ListTemplates error_type=DB_ERROR error=%q", err.Error()))
	}
	flash := r.URL.Query().Get("flash")

	// Resolve selected env
	var selectedEnv *models.Environment
	envIDStr := r.URL.Query().Get("env_id")
	if envIDStr != "" {
		envID, _ := strconv.ParseInt(envIDStr, 10, 64)
		for _, e := range envs {
			if e.ID == envID {
				selectedEnv = e
				break
			}
		}
	}

	// Load notes for env + template
	var envID int64
	if selectedEnv != nil {
		envID = selectedEnv.ID
	}
	var tmplID int64
	tmplIDStr := r.URL.Query().Get("template_id")
	if tmplIDStr != "" {
		tmplID, _ = strconv.ParseInt(tmplIDStr, 10, 64)
	}
	notes, err := h.DB.ListNotes(user.Username, envID, tmplID)
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Page DB error | op=ListNotes env_id=%d template_id=%d error_type=DB_ERROR error=%q",
				envID, tmplID, err.Error()))
	}

	// Resolve selected template
	var selectedTmpl *models.Template
	if tmplID > 0 {
		for _, t := range tmplList {
			if t.ID == tmplID {
				selectedTmpl = t
				break
			}
		}
	}

	h.render(w, r, "execution.html", &PageData{
		User:       user,
		ActiveMenu: "execution",
		Flash:      flash,
		Data: &executionPageData{
			Environments: envs,
			Templates:    tmplList,
			Notes:        notes,
			SelectedEnv:  selectedEnv,
			SelectedTmpl: selectedTmpl,
			CanRun:       user.HasRole(models.RoleAdmin, models.RoleStandard),
			CanAddNote:   user.HasRole(models.RoleAdmin, models.RoleStandard),
			CanEdit:      user.HasRole(models.RoleAdmin, models.RoleStandard),
		},
	})
}

func (h *ExecutionHandler) Run(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	envID, _ := strconv.ParseInt(r.FormValue("env_id"), 10, 64)
	tmplID, _ := strconv.ParseInt(r.FormValue("template_id"), 10, 64)

	tmpl, err := h.DB.GetTemplateByID(tmplID)
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Run DB error | op=GetTemplateByID template_id=%d error_type=DB_ERROR error=%q",
				tmplID, err.Error()))
	}
	if err != nil || tmpl == nil {
		flashRedirect(w, r, "/execution?env_id="+strconv.FormatInt(envID, 10), "Template not found")
		return
	}

	// Check restricted execution permission
	if tmpl.RestrictedExecution && user.HasRole(models.RoleRestricted) {
		flashRedirect(w, r, "/execution?env_id="+strconv.FormatInt(envID, 10), "You do not have permission to run this restricted template")
		return
	}
	if !user.HasRole(models.RoleAdmin, models.RoleStandard, models.RoleRestricted) {
		flashRedirect(w, r, "/execution?env_id="+strconv.FormatInt(envID, 10), "You do not have permission to run templates")
		return
	}

	// Substitute variables
	vars, err := h.DB.ListVariables()
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Run DB error | op=ListVariables error_type=DB_ERROR error=%q", err.Error()))
	}
	var envName string
	envs, err := h.DB.ListEnvironments()
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Run DB error | op=ListEnvironments error_type=DB_ERROR error=%q", err.Error()))
	}
	for _, e := range envs {
		if e.ID == envID {
			envName = e.Name
			break
		}
	}

	resolveVars := func(s string) string {
		for _, v := range vars {
			val := v.Values[envName]
			s = strings.ReplaceAll(s, "{{"+v.Name+"}}", val)
		}
		return s
	}

	// Compute reserved variable values once for this execution
	reservedVals := resolveReservedVarMap(user.Username, tmpl.ServiceName)
	resolveAll := func(s string) string {
		return applyReservedVars(resolveVars(s), reservedVals)
	}

	url := resolveAll(tmpl.URL)
	body := resolveAll(tmpl.Body)

	// Compute masked variants for logging (password values → ******)
	logURL := maskPasswordValues(url, vars, envName)
	logBody := maskPasswordValues(body, vars, envName)

	// Build request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(tmpl.Method, url, bodyReader)
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Run invalid request | template=%q method=%s url=%s error_type=REQUEST_ERROR error=%q",
				tmpl.Name, tmpl.Method, logURL, err.Error()))
		flashRedirect(w, r, "/execution?env_id="+strconv.FormatInt(envID, 10), "Invalid request: "+err.Error())
		return
	}

	for _, hdr := range tmpl.Headers {
		req.Header.Set(resolveAll(hdr.Key), resolveAll(hdr.Value))
	}

	// ── Log request start ─────────────────────────────────────────────────────
	h.techLog(r, logger.LevelInfo, "execution",
		fmt.Sprintf("Run start | template=%q env=%q method=%s url=%s body=%s",
			tmpl.Name, envName, tmpl.Method, logURL, logger.FormatBytes(len(logBody))))

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	result := &execResult{Duration: duration}
	if err != nil {
		errType := logger.ClassifyNetErr(err)
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("Run failed | template=%q env=%q method=%s url=%s error_type=%s duration=%s error=%q",
				tmpl.Name, envName, tmpl.Method, logURL, errType, duration.Round(time.Millisecond), err.Error()))
		result.Error = err.Error()
	} else {
		defer resp.Body.Close()
		result.StatusCode = resp.StatusCode
		result.Status = resp.Status
		result.Headers = map[string][]string(resp.Header)
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		result.Body = string(bodyBytes)

		httpErrType := logger.ClassifyHTTPStatus(resp.StatusCode)
		logLevel := logger.LevelInfo
		logMsg := fmt.Sprintf("Run success | template=%q env=%q method=%s url=%s status=%d duration=%s response=%s",
			tmpl.Name, envName, tmpl.Method, logURL, resp.StatusCode,
			duration.Round(time.Millisecond), logger.FormatBytes(len(bodyBytes)))
		if httpErrType == logger.ErrTypeHTTP5xx {
			logLevel = logger.LevelError
			logMsg = fmt.Sprintf("Run HTTP server error | template=%q env=%q method=%s url=%s status=%d error_type=%s duration=%s response=%s",
				tmpl.Name, envName, tmpl.Method, logURL, resp.StatusCode, httpErrType,
				duration.Round(time.Millisecond), logger.FormatBytes(len(bodyBytes)))
		} else if httpErrType == logger.ErrTypeHTTP4xx {
			logLevel = logger.LevelWarn
			logMsg = fmt.Sprintf("Run HTTP client error | template=%q env=%q method=%s url=%s status=%d error_type=%s duration=%s response=%s",
				tmpl.Name, envName, tmpl.Method, logURL, resp.StatusCode, httpErrType,
				duration.Round(time.Millisecond), logger.FormatBytes(len(bodyBytes)))
		}
		h.techLog(r, logLevel, "execution", logMsg)
	}

	h.log(r, "execution", fmt.Sprintf("Executed template '%s' in env '%s'", tmpl.Name, envName))

	tmplList, _ := h.DB.ListTemplates()
	notes, _ := h.DB.ListNotes(user.Username, envID, tmplID)

	var selectedEnv *models.Environment
	for _, e := range envs {
		if e.ID == envID {
			selectedEnv = e
			break
		}
	}

	h.render(w, r, "execution.html", &PageData{
		User:       user,
		ActiveMenu: "execution",
		Data: &executionPageData{
			Environments: envs,
			Templates:    tmplList,
			Notes:        notes,
			SelectedEnv:  selectedEnv,
			SelectedTmpl: tmpl,
			Result:       result,
			CanRun:       user.HasRole(models.RoleAdmin, models.RoleStandard),
			CanAddNote:   user.HasRole(models.RoleAdmin, models.RoleStandard),
		},
	})
}

func (h *ExecutionHandler) AddNote(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden"})
		return
	}
	// ParseMultipartForm handles both multipart/form-data (sent by JS FormData)
	// and application/x-www-form-urlencoded. Limit to 1 MB.
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		// Fall back to URL-encoded parse (e.g. plain form POST)
		if err2 := r.ParseForm(); err2 != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
			return
		}
	}
	envID, _ := strconv.ParseInt(r.FormValue("env_id"), 10, 64)
	tmplID, _ := strconv.ParseInt(r.FormValue("template_id"), 10, 64)
	if tmplID == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "template_id is required"})
		return
	}
	note := &models.Note{
		Title:         r.FormValue("title"),
		Body:          r.FormValue("body"),
		IsPrivate:     r.FormValue("is_private") == "1",
		OwnerUsername: user.Username,
	}
	if envID > 0 {
		note.EnvironmentID = &envID
	}
	note.TemplateID = &tmplID
	if err := h.DB.CreateNote(note); err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("AddNote DB error | op=CreateNote template_id=%d error_type=DB_ERROR error=%q",
				tmplID, err.Error()))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save note: " + err.Error()})
		return
	}
	h.log(r, "execution", "Added note: "+note.Title)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": note.ID})
}

func (h *ExecutionHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		_ = r.ParseForm()
	}
	noteID, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	isAdmin := user != nil && user.Role == "admin"
	if err := h.DB.DeleteNote(noteID, user.Username, isAdmin); err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("DeleteNote DB error | op=DeleteNote note_id=%d error_type=DB_ERROR error=%q",
				noteID, err.Error()))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	h.log(r, "execution", fmt.Sprintf("Deleted note %d", noteID))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// GetNotesJSON returns notes for a given env and template as JSON (for client-side rendering).
func (h *ExecutionHandler) GetNotesJSON(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	envID, _ := strconv.ParseInt(r.URL.Query().Get("env_id"), 10, 64)
	tmplID, _ := strconv.ParseInt(r.URL.Query().Get("template_id"), 10, 64)
	notes, err := h.DB.ListNotes(user.Username, envID, tmplID)
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("GetNotesJSON DB error | op=ListNotes env_id=%d template_id=%d error_type=DB_ERROR error=%q",
				envID, tmplID, err.Error()))
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if notes == nil {
		notes = []*models.Note{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

// GetTemplateJSON returns a template as JSON (for pre-filling the run form).
func (h *ExecutionHandler) GetTemplateJSON(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.ParseInt(idStr, 10, 64)
	t, err := h.DB.GetTemplateByID(id)
	if err != nil || t == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// maskPasswordValues replaces any password-variable values that appear literally
// in text with "******". Pass envName="" to scan values from all environments.
func maskPasswordValues(text string, vars []*models.Variable, envName string) string {
	for _, v := range vars {
		if !v.IsPassword {
			continue
		}
		if envName != "" {
			if val := v.Values[envName]; val != "" {
				text = strings.ReplaceAll(text, val, "******")
			}
		} else {
			for _, val := range v.Values {
				if val != "" {
					text = strings.ReplaceAll(text, val, "******")
				}
			}
		}
	}
	return text
}

// GetVariablesJSON returns all variables as JSON so the client can resolve
// {{placeholders}} before sending a run request.
func (h *ExecutionHandler) GetVariablesJSON(w http.ResponseWriter, r *http.Request) {
	vars, err := h.DB.ListVariables()
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("GetVariablesJSON DB error | op=ListVariables error_type=DB_ERROR error=%q", err.Error()))
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vars)
}

// runJSONReq is the JSON body expected by RunJSON.
type runJSONReq struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	ServiceName string            `json:"service_name"` // used for {{SERVICENAME}} resolution
}

// runJSONResp is what RunJSON returns to the client.
type runJSONResp struct {
	Request  *runJSONReq `json:"request"`
	Response *jsonResult `json:"response,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type jsonResult struct {
	StatusCode int                 `json:"status_code"`
	Status     string              `json:"status"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	Duration   string              `json:"duration"`
}

// RunJSON accepts a fully-resolved HTTP request as JSON, executes it, and
// returns both the echoed request and the response as JSON.
func (h *ExecutionHandler) RunJSON(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.CurrentUser(r)
	if !user.HasRole(models.RoleAdmin, models.RoleStandard, models.RoleRestricted) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req runJSONReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("RunJSON decode error: %s | error_type=REQUEST_ERROR", err.Error()))
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Load variables so we can mask password values in log messages.
	allVars, _ := h.DB.ListVariables()

	writeErr := func(msg string) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&runJSONResp{Request: &req, Error: msg})
	}

	// ── Log request start ─────────────────────────────────────────────────────
	headerCount := len(req.Headers)
	bodySize := len(req.Body)
	logURL := maskPasswordValues(req.URL, allVars, "")
	h.techLog(r, logger.LevelInfo, "execution",
		fmt.Sprintf("RunJSON start | method=%s url=%s headers=%d body=%s",
			strings.ToUpper(req.Method), logURL, headerCount, logger.FormatBytes(bodySize)))

	// Apply any remaining reserved-variable tokens (safety net for
	// tokens the client may not have substituted).
	reservedVals := resolveReservedVarMap(user.Username, req.ServiceName)
	req.URL = applyReservedVars(req.URL, reservedVals)
	req.Body = applyReservedVars(req.Body, reservedVals)
	for k, v := range req.Headers {
		newK := applyReservedVars(k, reservedVals)
		newV := applyReservedVars(v, reservedVals)
		if newK != k {
			delete(req.Headers, k)
		}
		req.Headers[newK] = newV
	}

	// Build body reader from the fully-resolved body (must be after reserved-var substitution).
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = bytes.NewBufferString(req.Body)
	}

	httpReq, err := http.NewRequest(strings.ToUpper(req.Method), req.URL, bodyReader)
	if err != nil {
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("RunJSON invalid request | method=%s url=%s error_type=REQUEST_ERROR error=%q",
				strings.ToUpper(req.Method), logURL, err.Error()))
		writeErr("invalid request: " + err.Error())
		return
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(httpReq)
	dur := time.Since(start)

	out := &runJSONResp{Request: &req}
	if err != nil {
		errType := logger.ClassifyNetErr(err)
		h.techLog(r, logger.LevelError, "execution",
			fmt.Sprintf("RunJSON failed | method=%s url=%s error_type=%s duration=%s error=%q",
				strings.ToUpper(req.Method), logURL, errType, dur.Round(time.Millisecond), err.Error()))
		out.Error = err.Error()
	} else {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		out.Response = &jsonResult{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Headers:    map[string][]string(resp.Header),
			Body:       string(bodyBytes),
			Duration:   dur.String(),
		}

		// ── Log response ──────────────────────────────────────────────────────
		httpErrType := logger.ClassifyHTTPStatus(resp.StatusCode)
		logLevel := logger.LevelInfo
		logMsg := fmt.Sprintf("RunJSON success | method=%s url=%s status=%d duration=%s response=%s",
			strings.ToUpper(req.Method), logURL, resp.StatusCode,
			dur.Round(time.Millisecond), logger.FormatBytes(len(bodyBytes)))
		if httpErrType == logger.ErrTypeHTTP5xx {
			logLevel = logger.LevelError
			logMsg = fmt.Sprintf("RunJSON HTTP server error | method=%s url=%s status=%d error_type=%s duration=%s response=%s",
				strings.ToUpper(req.Method), logURL, resp.StatusCode, httpErrType,
				dur.Round(time.Millisecond), logger.FormatBytes(len(bodyBytes)))
		} else if httpErrType == logger.ErrTypeHTTP4xx {
			logLevel = logger.LevelWarn
			logMsg = fmt.Sprintf("RunJSON HTTP client error | method=%s url=%s status=%d error_type=%s duration=%s response=%s",
				strings.ToUpper(req.Method), logURL, resp.StatusCode, httpErrType,
				dur.Round(time.Millisecond), logger.FormatBytes(len(bodyBytes)))
		}
		h.techLog(r, logLevel, "execution", logMsg)
	}

	statusCode := 0
	if out.Response != nil {
		statusCode = out.Response.StatusCode
	}
	h.log(r, "execution", fmt.Sprintf("RunJSON %s %s → %d", req.Method, logURL, statusCode))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
