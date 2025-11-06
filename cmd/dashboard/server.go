package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"ship-status-dash/pkg/types"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Server represents the HTTP server for the dashboard API.
type Server struct {
	logger     *logrus.Logger
	config     *types.Config
	handlers   *Handlers
	db         *gorm.DB
	corsOrigin string
	hmacSecret []byte
	httpServer *http.Server
}

// NewServer creates a new Server instance
func NewServer(config *types.Config, db *gorm.DB, logger *logrus.Logger, corsOrigin string, hmacSecret []byte) *Server {
	return &Server{
		logger:     logger,
		config:     config,
		handlers:   NewHandlers(logger, config, db),
		db:         db,
		corsOrigin: corsOrigin,
		hmacSecret: hmacSecret,
	}
}

func (s *Server) setupRoutes() http.Handler {
	router := mux.NewRouter()

	router.HandleFunc("/health", s.handlers.HealthJSON).Methods(http.MethodGet)

	router.HandleFunc("/api/status", s.handlers.GetAllComponentsStatusJSON).Methods(http.MethodGet)
	router.HandleFunc("/api/status/{componentName}", s.handlers.GetComponentStatusJSON).Methods(http.MethodGet)
	router.HandleFunc("/api/status/{componentName}/{subComponentName}", s.handlers.GetSubComponentStatusJSON).Methods(http.MethodGet)

	router.HandleFunc("/api/components", s.handlers.GetComponentsJSON).Methods(http.MethodGet)
	router.HandleFunc("/api/components/{componentName}", s.handlers.GetComponentInfoJSON).Methods(http.MethodGet)
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}", s.handlers.GetOutageJSON).Methods(http.MethodGet)
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}", s.handlers.UpdateOutageJSON).Methods(http.MethodPatch)
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}", s.handlers.DeleteOutage).Methods(http.MethodDelete)
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages", s.handlers.CreateOutageJSON).Methods(http.MethodPost)
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages", s.handlers.GetSubComponentOutagesJSON).Methods(http.MethodGet)
	router.HandleFunc("/api/components/{componentName}/outages", s.handlers.GetOutagesJSON).Methods(http.MethodGet)

	// Serve static files (React frontend) - must be after API routes
	spa := spaHandler{staticPath: "./static", indexPath: "index.html"}
	router.PathPrefix("/").Handler(spa)

	authHandler := newAuthMiddleware(s.logger, s.hmacSecret, router)
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{s.corsOrigin}),
		handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Forwarded-User", "GAP-Signature"}),
		handlers.AllowCredentials(),
	)(authHandler)

	handler := s.loggingMiddleware(corsHandler)

	return handler
}

// spaHandler implements the http.Handler interface for serving a Single Page Application.
// It serves static files if they exist, otherwise serves index.html to allow
// client-side routing to work.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.staticPath, r.URL.Path)

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}

	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		s.logger.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"duration": time.Since(start),
		}).Info("Request processed")
	})
}

// Start begins listening for HTTP requests on the specified address.
func (s *Server) Start(addr string) error {
	handler := s.setupRoutes()
	s.logger.Infof("Starting dashboard server on %s", addr)
	s.httpServer = &http.Server{Addr: addr, Handler: handler}
	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.logger.Info("Shutting down dashboard server")
	return s.httpServer.Shutdown(ctx)
}
