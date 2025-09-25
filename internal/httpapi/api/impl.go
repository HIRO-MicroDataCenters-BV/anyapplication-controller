package api

import (
	"encoding/json"
	"log"
	"net/http"

	v1 "hiro.io/anyapplication/api/v1"
	ctrltypes "hiro.io/anyapplication/internal/controller/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationApiOptions struct {
	Address string
}

// ensure that we've conformed to the `ServerInterface` with a compile-time check
var _ ServerInterface = (*ServerImpl)(nil)

type ServerImpl struct {
	applicationReports ApplicationReports
	applications       ctrltypes.Applications
	applicationSpecs   ApplicationSpecs
	kubeClient         client.Client
}

func NewServer(
	applicationReports ApplicationReports,
	applicationSpecs ApplicationSpecs,
	applications ctrltypes.Applications,
	kubeClient client.Client,
) ServerInterface {
	return ServerImpl{
		applicationReports: applicationReports,
		applicationSpecs:   applicationSpecs,
		applications:       applications,
		kubeClient:         kubeClient,
	}
}

func (s ServerImpl) GetApplicationStatus(w http.ResponseWriter, r *http.Request, namespace string, name string) {
	application := &v1.AnyApplication{}
	if err := s.kubeClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, application); err != nil {
		s.replyError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	instanceId := s.applications.GetInstanceId(application)

	report, err := s.applicationReports.Fetch(r.Context(), instanceId, namespace)
	if err != nil {
		s.replyError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(report); err != nil {
		log.Printf("failed to encode: %s", err)
	}

}

func (s ServerImpl) GetApplicationSpec(w http.ResponseWriter, r *http.Request, namespace string, name string) {
	application := &v1.AnyApplication{}
	if err := s.kubeClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, application); err != nil {
		s.replyError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	applicationSpec, err := s.applicationSpecs.GetApplicationSpec(r.Context(), application)
	if err != nil {
		s.replyError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(applicationSpec); err != nil {
		log.Printf("failed to encode: %s", err)
	}

}

func (s ServerImpl) replyError(w http.ResponseWriter, status int, code string, msg string) {
	response := ErrorResponse{
		Status:  status,
		Code:    code,
		Message: msg,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode: %s", err)
	}
}
