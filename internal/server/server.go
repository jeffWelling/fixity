package server

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jeffanddom/fixity/internal/auth"
	"github.com/jeffanddom/fixity/internal/coordinator"
	"github.com/jeffanddom/fixity/internal/database"
)

// Server handles HTTP requests
type Server struct {
	db          *database.Database
	auth        *auth.Service
	coordinator *coordinator.Coordinator
	router      *chi.Mux
	templates   *template.Template
	config      Config
}

// Config holds server configuration
type Config struct {
	ListenAddr      string
	SessionCookieName string
	TemplateDir     string
	StaticDir       string
}

// New creates a new HTTP server
func New(
	db *database.Database,
	authService *auth.Service,
	coord *coordinator.Coordinator,
	config Config,
) (*Server, error) {
	// Set defaults
	if config.ListenAddr == "" {
		config.ListenAddr = ":8080"
	}
	if config.SessionCookieName == "" {
		config.SessionCookieName = "fixity_session"
	}

	s := &Server{
		db:          db,
		auth:        authService,
		coordinator: coord,
		config:      config,
	}

	// Load templates if directory specified
	if config.TemplateDir != "" {
		tmpl, err := template.ParseGlob(config.TemplateDir + "/*.html")
		if err != nil {
			return nil, fmt.Errorf("failed to parse templates: %w", err)
		}
		s.templates = tmpl
	}

	// Setup routes
	s.setupRoutes()

	return s, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	// Static files (if configured)
	if s.config.StaticDir != "" {
		r.Handle("/static/*", http.StripPrefix("/static/",
			http.FileServer(http.Dir(s.config.StaticDir))))
	}

	// Public routes (no authentication required)
	r.Group(func(r chi.Router) {
		r.Get("/login", s.handleLoginPage)
		r.Post("/login", s.handleLogin)
		r.Get("/health", s.handleHealth)
	})

	// Protected routes (authentication required)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Get("/", s.handleDashboard)
		r.Post("/logout", s.handleLogout)

		// Storage targets
		r.Route("/targets", func(r chi.Router) {
			r.Get("/", s.handleListTargets)
			r.Get("/new", s.handleNewTargetPage)
			r.Post("/", s.handleCreateTarget)
			r.Get("/{id}", s.handleViewTarget)
			r.Get("/{id}/edit", s.handleEditTargetPage)
			r.Put("/{id}", s.handleUpdateTarget)
			r.Delete("/{id}", s.handleDeleteTarget)
			r.Post("/{id}/scan", s.handleTriggerScan)
		})

		// Scans
		r.Route("/scans", func(r chi.Router) {
			r.Get("/", s.handleListScans)
			r.Get("/{id}", s.handleViewScan)
			r.Get("/running", s.handleRunningScans)
		})

		// Files
		r.Route("/files", func(r chi.Router) {
			r.Get("/", s.handleBrowseFiles)
			r.Get("/{id}", s.handleViewFile)
			r.Get("/{id}/history", s.handleFileHistory)
		})

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)

			r.Route("/users", func(r chi.Router) {
				r.Get("/", s.handleListUsers)
				r.Get("/new", s.handleNewUserPage)
				r.Post("/", s.handleCreateUser)
				r.Get("/{id}", s.handleViewUser)
				r.Delete("/{id}", s.handleDeleteUser)
			})
		})
	})

	s.router = r
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return http.ListenAndServe(s.config.ListenAddr, s.router)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Server is using http.ListenAndServe, so we can't gracefully shutdown
	// In production, use http.Server directly with Shutdown method
	return nil
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
