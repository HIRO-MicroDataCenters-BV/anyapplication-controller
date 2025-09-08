// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	v1 "hiro.io/anyapplication/api/v1"
	ctrltypes "hiro.io/anyapplication/internal/controller/types"
	types "hiro.io/anyapplication/internal/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationApiOptions struct {
	Address string
}

type Server struct {
	mux                *http.ServeMux
	options            ApplicationApiOptions
	applicationReports types.ApplicationReports
	applications       ctrltypes.Applications
	kubeClient         client.Client
}

// NewServer creates and configures a new Server
func NewHttpServer(
	options ApplicationApiOptions,
	applicationReports types.ApplicationReports,
	applications *ctrltypes.Applications,
	kubeClient client.Client,
) *Server {
	s := &Server{
		mux:                http.NewServeMux(),
		options:            options,
		applicationReports: applicationReports,
		applications:       *applications,
		kubeClient:         kubeClient,
	}
	s.routes()
	return s
}

// routes sets up the HTTP routes
func (s *Server) routes() {
	router := chi.NewRouter()
	router.Get("/status/{namespace}/{name}", s.handleGetApplicationErrorContext)
	s.mux.Handle("/", router)
}

func (s *Server) handleGetApplicationErrorContext(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	if namespace == "" || name == "" {
		http.Error(w, "Namespace and name are required", http.StatusBadRequest)
		return
	}

	application := &v1.AnyApplication{}
	if err := s.kubeClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, application); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	instanceId := s.applications.GetInstanceId(application)

	report, err := s.applicationReports.Fetch(r.Context(), instanceId, namespace)
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
