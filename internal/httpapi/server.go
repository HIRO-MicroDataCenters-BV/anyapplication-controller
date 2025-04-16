package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

type ApplicationApiOptions struct {
	Address string
}

type Server struct {
	mux     *http.ServeMux
	options ApplicationApiOptions
}

// NewServer creates and configures a new Server
func NewHttpServer(options ApplicationApiOptions) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		options: options,
	}
	s.routes()
	return s
}

// routes sets up the HTTP routes
func (s *Server) routes() {
	s.mux.HandleFunc("/application", s.handleGetApplication)
}

func (s *Server) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	obj := ApplicationBundle{
		ID:   1,
		Name: "Sample Object",
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(obj); err != nil {
		log.Printf("failed to encode: %s", err)
	}
}

// Start runs the HTTP server
func (s *Server) Start() error {
	log.Printf("API Server is running at %s\n", s.options.Address)
	return http.ListenAndServe(s.options.Address, s.mux)
}
