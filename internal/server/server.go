package server

import (
	"fmt"
	"net/http"

	"github.com/driversti/keyforge/internal/api"
	"github.com/driversti/keyforge/internal/auth"
	"github.com/driversti/keyforge/internal/db"
)

// Server holds the HTTP server dependencies and routes.
type Server struct {
	db         *db.DB
	apiHandler *api.Handler
	apiKey     string
	mux        *http.ServeMux
}

// New creates a new Server, wires up all routes, and returns it.
func New(database *db.DB, apiKey string) *Server {
	s := &Server{
		db:         database,
		apiHandler: api.NewHandler(database),
		apiKey:     apiKey,
		mux:        http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	requireKey := auth.RequireAPIKey(s.apiKey)

	// Public routes (no auth).
	s.mux.HandleFunc("GET /api/v1/authorized_keys", s.apiHandler.GetAuthorizedKeys)
	s.mux.HandleFunc("GET /api/v1/health", s.apiHandler.HealthCheck)

	// Protected routes.
	s.mux.Handle("GET /api/v1/devices", requireKey(http.HandlerFunc(s.apiHandler.ListDevices)))
	s.mux.Handle("POST /api/v1/devices", requireKey(http.HandlerFunc(s.apiHandler.CreateDevice)))

	s.mux.Handle("GET /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.GetDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("PATCH /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.UpdateDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("DELETE /api/v1/devices/{id}", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.DeleteDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /api/v1/devices/{id}/revoke", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.RevokeDevice(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /api/v1/devices/{id}/reactivate", requireKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.apiHandler.ReactivateDevice(w, r, r.PathValue("id"))
	})))
}

// ServeHTTP implements http.Handler, allowing Server to be used directly in tests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on the given port.
func (s *Server) ListenAndServe(port int) error {
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("KeyForge server listening on %s\n", addr)
	return http.ListenAndServe(addr, s.mux)
}
