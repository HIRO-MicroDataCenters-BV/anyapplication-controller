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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	instanceId := s.applications.GetInstanceId(application)

	report, err := s.applicationReports.Fetch(r.Context(), instanceId, namespace)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(report); err != nil {
		log.Printf("failed to encode: %s", err)
	}

}

func (s ServerImpl) GetApplicationSpec(w http.ResponseWriter, r *http.Request, namespace string, name string) {
	application := &v1.AnyApplication{}
	if err := s.kubeClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, application); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	applicationSpec, err := s.applicationSpecs.GetApplicationSpec(r.Context(), application)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(applicationSpec); err != nil {
		log.Printf("failed to encode: %s", err)
	}

}
