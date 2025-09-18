// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"log"
	"net/http"

	ctrltypes "hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/httpapi/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationApiOptions struct {
	Address string
}

type ApiServer struct {
	server  *http.Server
	options ApplicationApiOptions
}

// NewServer creates and configures a new Server
func NewHttpServer(
	options ApplicationApiOptions,
	applicationReports api.ApplicationReports,
	applications *ctrltypes.Applications,
	kubeClient client.Client,
) *ApiServer {
	serverImpl := api.NewServer(applicationReports, *applications, kubeClient)
	r := http.NewServeMux()
	// get an `http.Handler` that we can use
	httpHandler := api.HandlerFromMux(serverImpl, r)
	httpServer := &http.Server{
		Handler: httpHandler,
		Addr:    options.Address,
	}
	server := ApiServer{
		server:  httpServer,
		options: options,
	}
	return &server
}

// Start runs the HTTP server
func (s *ApiServer) Start() error {
	log.Printf("API Server is running at %s\n", s.options.Address)
	return s.server.ListenAndServe()
}
