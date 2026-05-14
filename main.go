package main

import (
	"apicalls/auth"
	"apicalls/config"
	"apicalls/handlers"
	"apicalls/logger"
	"apicalls/storage"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// Version and BuildTime can be overridden at link time:
//
//	go build -ldflags "-X main.Version=1.2.3 -X main.BuildTime=2026-03-05T10:00:00Z"
var (
	Version   = "1.0.0"
	BuildTime = "" // set via ldflags; falls back to startup time
)

func main() {
	if BuildTime == "" {
		BuildTime = time.Now().Format("2006-01-02 15:04:05")
	}
	handlers.AppVersion = Version
	handlers.AppBuildTime = BuildTime
	// ── Config ──────────────────────────────────
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// ── Logger ──────────────────────────────────
	if err := logger.Init(cfg.Log.Path); err != nil {
		log.Fatalf("init logger: %v", err)
	}
	if err := logger.TechInit(cfg.Log.TechPath); err != nil {
		log.Fatalf("init techlog: %v", err)
	}

	// ── Database ────────────────────────────────
	db, err := storage.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// ── Auth ────────────────────────────────────
	authMgr := auth.NewManager(cfg.Session.Secret, cfg.Session.Name, cfg.Session.MaxAge, db)

	// ── Shared base ─────────────────────────────
	base := handlers.Base{
		DB:      db,
		Auth:    authMgr,
		TmplDir: "templates",
	}

	// ── Handlers ────────────────────────────────
	authH := &handlers.AuthHandler{Base: base}
	envH := &handlers.EnvironmentHandler{Base: base}
	varH := &handlers.VariableHandler{Base: base}
	resvH := &handlers.ReservedVariableHandler{Base: base}
	techH := &handlers.TechnologyHandler{Base: base}
	tmplH := &handlers.TemplateHandler{Base: base}
	execH := &handlers.ExecutionHandler{Base: base}
	userH := &handlers.UsersHandler{Base: base}

	// ── Router ──────────────────────────────────
	r := mux.NewRouter()

	// Static assets
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Auth routes (public)
	r.HandleFunc("/login", authH.LoginPage).Methods("GET")
	r.HandleFunc("/login", authH.LoginPost).Methods("POST")
	r.HandleFunc("/logout", authH.Logout)

	// Protected routes
	protected := r.PathPrefix("").Subrouter()
	protected.Use(authMgr.RequireLogin)

	protected.HandleFunc("/", authH.Dashboard).Methods("GET")

	// Environments
	protected.HandleFunc("/environments", envH.List).Methods("GET")
	protected.HandleFunc("/environments/create", envH.Create).Methods("POST")
	protected.HandleFunc("/environments/update", envH.Update).Methods("POST")
	protected.HandleFunc("/environments/delete", envH.Delete).Methods("POST")

	// Variables
	protected.HandleFunc("/variables", varH.List).Methods("GET")
	protected.HandleFunc("/variables/create", varH.Create).Methods("POST")
	protected.HandleFunc("/variables/update", varH.Update).Methods("POST")
	protected.HandleFunc("/variables/delete", varH.Delete).Methods("POST")

	// Reserved Variables
	protected.HandleFunc("/reserved-variables", resvH.Page).Methods("GET")
	protected.HandleFunc("/reserved-variables/update-description", resvH.UpdateDescription).Methods("POST")
	protected.HandleFunc("/api/reserved-variables/resolve", resvH.GetReservedVariablesJSON).Methods("GET")

	// Technologies
	protected.HandleFunc("/technologies", techH.List).Methods("GET")
	protected.HandleFunc("/technologies/create", techH.Create).Methods("POST")
	protected.HandleFunc("/technologies/update", techH.Update).Methods("POST")
	protected.HandleFunc("/technologies/delete", techH.Delete).Methods("POST")
	protected.HandleFunc("/api/technologies/{id}", techH.GetTechnologyJSON).Methods("GET")

	// Templates
	protected.HandleFunc("/templates", tmplH.List).Methods("GET")
	protected.HandleFunc("/templates/create", tmplH.Create).Methods("POST")
	protected.HandleFunc("/templates/update", tmplH.Update).Methods("POST")
	protected.HandleFunc("/templates/delete", tmplH.Delete).Methods("POST")
	protected.HandleFunc("/templates/save-copy", tmplH.SaveCopy).Methods("POST")
	protected.HandleFunc("/api/templates/unique-name", tmplH.UniqueName).Methods("GET")
	protected.HandleFunc("/api/templates/parse-wsdl", tmplH.ParseWSDL).Methods("POST")
	protected.HandleFunc("/api/templates/{id}", execH.GetTemplateJSON).Methods("GET")

	// Execution
	protected.HandleFunc("/execution", execH.Page).Methods("GET")
	protected.HandleFunc("/execution/run", execH.Run).Methods("POST")
	protected.HandleFunc("/execution/notes/add", execH.AddNote).Methods("POST")
	protected.HandleFunc("/execution/notes/delete", execH.DeleteNote).Methods("POST")
	protected.HandleFunc("/api/variables", execH.GetVariablesJSON).Methods("GET")
	protected.HandleFunc("/api/execution/run", execH.RunJSON).Methods("POST")
	protected.HandleFunc("/api/notes", execH.GetNotesJSON).Methods("GET")

	// Users (admin only – enforced inside handler)
	protected.HandleFunc("/users", userH.List).Methods("GET")
	protected.HandleFunc("/users/create", userH.Create).Methods("POST")
	protected.HandleFunc("/users/update", userH.Update).Methods("POST")
	protected.HandleFunc("/users/delete", userH.Delete).Methods("POST")

	// ── Server ──────────────────────────────────
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("APICalls starting on https://%s\n", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	if err := srv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
