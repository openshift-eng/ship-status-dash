package main

import (
	"context"
	"net/http"
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
	httpServer *http.Server
}

// NewServer creates a new Server instance with the provided configuration, database connection, and logger.
func NewServer(config *types.Config, db *gorm.DB, logger *logrus.Logger, corsOrigin string) *Server {
	handlers := NewHandlers(logger, config, db)

	return &Server{
		logger:     logger,
		config:     config,
		handlers:   handlers,
		db:         db,
		corsOrigin: corsOrigin,
	}
}

func (s *Server) setupRoutes() http.Handler {
	router := mux.NewRouter()

	router.HandleFunc("/health", s.handlers.HealthJSON).Methods("GET")

	router.HandleFunc("/api/status", s.handlers.GetAllComponentsStatusJSON).Methods("GET")
	router.HandleFunc("/api/status/{componentName}", s.handlers.GetComponentStatusJSON).Methods("GET")
	router.HandleFunc("/api/status/{componentName}/{subComponentName}", s.handlers.GetSubComponentStatusJSON).Methods("GET")

	router.HandleFunc("/api/components", s.handlers.GetComponentsJSON).Methods("GET")
	router.HandleFunc("/api/components/{componentName}", s.handlers.GetComponentInfoJSON).Methods("GET")
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}", s.handlers.GetOutageJSON).Methods("GET")
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}", s.handlers.UpdateOutageJSON).Methods("PATCH")
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}", s.handlers.DeleteOutage).Methods("DELETE")
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages", s.handlers.CreateOutageJSON).Methods("POST")
	router.HandleFunc("/api/components/{componentName}/{subComponentName}/outages", s.handlers.GetSubComponentOutagesJSON).Methods("GET")
	router.HandleFunc("/api/components/{componentName}/outages", s.handlers.GetOutagesJSON).Methods("GET")

	// Serve static files (React frontend) - must be after API routes
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{s.corsOrigin}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Forwarded-User"}),
		handlers.AllowCredentials(),
	)(router)

	// Add logging middleware
	handler := s.loggingMiddleware(corsHandler)

	return handler
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
