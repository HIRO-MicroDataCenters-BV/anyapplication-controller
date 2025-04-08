/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AnyApplicationSpec defines the desired state of AnyApplication.
type AnyApplicationSpec struct {
	// Foo is an example field of AnyApplication. Edit anyapplication_types.go to remove/update
	Application     ApplicationMatcherSpec `json:"application,omitempty"`
	Zones           int                    `json:"zones,omitempty"`
	RecoverStrategy RecoverStrategySpec    `json:"recover-strategy,omitempty"`
}

type ApplicationMatcherSpec struct {
	ResourceSelector map[string]string `json:"resourceSelector,omitempty"`
}

type RecoverStrategySpec struct {
	Tolerance  int `json:"tolerance,omitempty"`
	MaxRetries int `json:"max-retries,omitempty"`
}

// AnyApplicationStatus defines the observed state of AnyApplication.
type AnyApplicationStatus struct {
	State      string            `json:"state,omitempty"`
	Placements []PlacementStatus `json:"placements,omitempty"`
	Owner      string            `json:"owner,omitempty"`
	Conditions []ConditionStatus `json:"conditions,omitempty"`
}

type PlacementStatus struct {
	Zone         string   `json:"zone,omitempty"`
	NodeAffinity []string `json:"node-affinity,omitempty"`
}

type ConditionStatus struct {
	Type               string      `json:"type,omitempty"`
	ZoneId             string      `json:"zoneId,omitempty"`
	Status             string      `json:"status,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	Reason             string      `json:"reason,omitempty"`
	Msg                string      `json:"msg,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AnyApplication is the Schema for the anyapplications API.
type AnyApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnyApplicationSpec   `json:"spec,omitempty"`
	Status AnyApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AnyApplicationList contains a list of AnyApplication.
type AnyApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AnyApplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AnyApplication{}, &AnyApplicationList{})
}
