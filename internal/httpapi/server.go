package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	types "hiro.io/anyapplication/internal/types"
)

type ApplicationApiOptions struct {
	Address string
}

type Server struct {
	mux                *http.ServeMux
	options            ApplicationApiOptions
	applicationReports types.ApplicationReports
}

// NewServer creates and configures a new Server
func NewHttpServer(options ApplicationApiOptions, applicationReports types.ApplicationReports) *Server {
	s := &Server{
		mux:                http.NewServeMux(),
		options:            options,
		applicationReports: applicationReports,
	}
	s.routes()
	return s
}

// routes sets up the HTTP routes
func (s *Server) routes() {
	s.mux.HandleFunc("/status", s.handleGetApplicationErrorContext)
}

func (s *Server) handleGetApplicationErrorContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	report, err := s.applicationReports.Fetch(r.Context(), "instanceId", "namespace")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(report); err != nil {
		log.Printf("failed to encode: %s", err)
	}
}

// Start runs the HTTP server
func (s *Server) Start() error {
	log.Printf("API Server is running at %s\n", s.options.Address)
	return http.ListenAndServe(s.options.Address, s.mux)
}
