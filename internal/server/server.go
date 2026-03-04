package server

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/driversti/keyforge/internal/api"
	"github.com/driversti/keyforge/internal/auth"
	"github.com/driversti/keyforge/internal/db"
	"github.com/driversti/keyforge/internal/web"
)

// Server holds the HTTP server dependencies and routes.
type Server struct {
	db         *db.DB
	apiHandler *api.Handler
	webHandler *web.Handler
	apiKey     string
	sessions   *web.SessionStore
	mux        *http.ServeMux
}

// New creates a new Server, wires up all routes, and returns it.
func New(database *db.DB, apiKey string, serverURL string) (*Server, error) {
	sessions := web.NewSessionStore()
	s := &Server{
		db:         database,
		apiHandler: api.NewHandler(database),
		webHandler: web.NewHandler(database, serverURL, apiKey, sessions),
		apiKey:     apiKey,
		sessions:   sessions,
		mux:        http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	requireKey := auth.RequireAPIKey(s.apiKey)
	requireSession := web.SessionAuth(s.apiKey, s.sessions)

	// Static files (no auth).
	staticFS, _ := fs.Sub(web.Content, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Login/Logout routes (no session auth).
	s.mux.HandleFunc("GET /login", s.webHandler.LoginPage)
	s.mux.HandleFunc("POST /login", s.webHandler.LoginSubmit)
	s.mux.HandleFunc("GET /logout", s.webHandler.LogoutHandler)

	// Web UI routes (session auth required).
	s.mux.Handle("GET /{$}", requireSession(http.HandlerFunc(s.webHandler.DevicesPage)))
	s.mux.Handle("GET /add", requireSession(http.HandlerFunc(s.webHandler.AddDevicePage)))
	s.mux.Handle("POST /add", requireSession(http.HandlerFunc(s.webHandler.AddDeviceSubmit)))
	s.mux.Handle("GET /authorized-keys", requireSession(http.HandlerFunc(s.webHandler.AuthorizedKeysPage)))
	s.mux.Handle("POST /devices/{id}/revoke", requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.webHandler.RevokeDeviceAction(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /devices/{id}/reactivate", requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.webHandler.ReactivateDeviceAction(w, r, r.PathValue("id"))
	})))
	s.mux.Handle("POST /devices/{id}/delete", requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.webHandler.DeleteDeviceAction(w, r, r.PathValue("id"))
	})))

	// Public API routes (no auth).
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
