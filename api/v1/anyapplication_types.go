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
	"fmt"
	"strconv"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AnyApplicationSpec defines the desired state of AnyApplication.
type AnyApplicationSpec struct {
	// Foo is an example field of AnyApplication. Edit anyapplication_types.go to remove/update
	Application       ApplicationMatcherSpec `json:"application" validate:"required"`
	Zones             int                    `json:"zones"`
	PlacementStrategy PlacementStrategySpec  `json:"placement-strategy,omitempty"`
	RecoverStrategy   RecoverStrategySpec    `json:"recover-strategy,omitempty"`
}

type ApplicationMatcherSpec struct {
	ResourceSelector *map[string]string `json:"resourceSelector,omitempty"`
	HelmSelector     *HelmSelectorSpec  `json:"helm,omitempty"`
}

type HelmSelectorSpec struct {
	Repository string `json:"repository"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	Namespace  string `json:"namespace"`
	Values     string `json:"values,omitempty"`
}
type PlacementStrategySpec struct {
	Strategy PlacementStrategy `json:"strategy"`
}

type RecoverStrategySpec struct {
	Tolerance  int `json:"tolerance,omitempty"`
	MaxRetries int `json:"max-retries,omitempty"`
}

// AnyApplicationStatus defines the observed state of AnyApplication.
type AnyApplicationStatus struct {
	State      GlobalState  `json:"state"`
	Owner      string       `json:"owner"`
	Placements []Placement  `json:"placements,omitempty"`
	Zones      []ZoneStatus `json:"zones,omitempty"`
}

type ZoneStatus struct {
	ZoneId      string            `json:"zoneId"`
	ZoneVersion int64             `json:"version"`
	Conditions  []ConditionStatus `json:"conditions,omitempty"`
}

type Placement struct {
	Zone         string   `json:"zone"`
	NodeAffinity []string `json:"node-affinity,omitempty"`
}

type ConditionStatus struct {
	Type               ApplicationConditionType `json:"type"`
	ZoneId             string                   `json:"zoneId"`
	Status             string                   `json:"status"`
	LastTransitionTime metav1.Time              `json:"lastTransitionTime"`
	Reason             string                   `json:"reason,omitempty"`
	Msg                string                   `json:"msg,omitempty"`
	RetryAttempt       int                      `json:"retryAttempt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AnyApplication is the Schema for the anyapplications API.
type AnyApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnyApplicationSpec   `json:"spec"`
	Status AnyApplicationStatus `json:"status,omitempty"`
}

func (a *AnyApplication) GetNamespacedName() client.ObjectKey {
	return client.ObjectKey{
		Namespace: a.Namespace,
		Name:      a.Name,
	}
}

func (g *AnyApplication) HasZoneStatus(zoneId string) bool {
	for _, zone := range g.Status.Zones {
		if zone.ZoneId == zoneId {
			return true
		}
	}
	return false
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

func (status *AnyApplicationStatus) GetStatusFor(zone string) (*ZoneStatus, bool) {
	for i, zoneStatus := range status.Zones {
		if zoneStatus.ZoneId == zone {
			return &status.Zones[i], true
		}
	}
	return nil, false
}

func (status *AnyApplicationStatus) GetOrCreateStatusFor(zone string) *ZoneStatus {
	for i, zoneStatus := range status.Zones {
		if zoneStatus.ZoneId == zone {
			return &status.Zones[i]
		}
	}
	newStatus := ZoneStatus{
		ZoneId:      zone,
		ZoneVersion: 0,
		Conditions:  make([]ConditionStatus, 0),
	}
	status.Zones = append(status.Zones, newStatus)
	for i, zoneStatus := range status.Zones {
		if zoneStatus.ZoneId == zone {
			return &status.Zones[i]
		}
	}
	return nil
}

func (status *AnyApplicationStatus) RemoveZone(zone string) {
	status.Zones = lo.Filter(status.Zones, func(existing ZoneStatus, _ int) bool {
		equal := existing.ZoneId == zone
		return !equal
	})
}

func (status *AnyApplicationStatus) AddOrUpdate(toAddOrUpdate *ConditionStatus, zoneId string) bool {
	zoneStatus := status.GetOrCreateStatusFor(zoneId)

	updated := false
	found := false

	for i, cond := range zoneStatus.Conditions {
		if cond.Type == toAddOrUpdate.Type && cond.ZoneId == toAddOrUpdate.ZoneId {
			found = true
			if cond.Status != toAddOrUpdate.Status || cond.Reason != toAddOrUpdate.Reason || cond.Msg != toAddOrUpdate.Msg {
				zoneStatus.Conditions[i] = *toAddOrUpdate
				updated = true
			}
			break
		}
	}

	if !found {
		zoneStatus.Conditions = append(zoneStatus.Conditions, *toAddOrUpdate)
		updated = true
	}
	return updated
}

func (status *AnyApplicationStatus) Remove(condType ApplicationConditionType, zoneId string) bool {
	zoneStatus, found := status.GetStatusFor(zoneId)
	if !found {
		return false
	}
	originalSize := len(zoneStatus.Conditions)
	zoneStatus.Conditions = lo.Filter(zoneStatus.Conditions, func(existing ConditionStatus, _ int) bool {
		equal := existing.Type == condType && existing.ZoneId == zoneId
		return !equal
	})

	return len(zoneStatus.Conditions) != originalSize
}

func (application *AnyApplication) IncrementZoneVersion(zoneId string) {
	zoneStatus, found := application.Status.GetStatusFor(zoneId)
	if !found {
		return
	}
	version, err := strconv.ParseInt(application.ResourceVersion, 10, 64)
	if err != nil {
		version = 0
	}
	latestVersion := version + 1
	if zoneStatus.ZoneVersion >= latestVersion {
		latestVersion = zoneStatus.ZoneVersion + 1
	}

	zoneStatus.ZoneVersion = latestVersion
}

func (status *AnyApplicationStatus) LogStatus() {
	out := "- status update -\n"
	for _, zone := range status.Zones {
		out += fmt.Sprintf(" zone: %v\n", zone.ZoneId)
		out += fmt.Sprintf("  - version: %v\n", zone.ZoneVersion)
		out += "  - conditions:\n"
		for _, cond := range zone.Conditions {
			out += fmt.Sprintf("   -- %v, %v\n", cond.Type, cond.Status)
		}
	}
	out += "\n"
	fmt.Print(out)
}

func (status *AnyApplicationStatus) ZoneExists(zone string) bool {
	_, exists := status.GetStatusFor(zone)
	return exists
}

func (status *ZoneStatus) FindCondition(conditionType ApplicationConditionType) (*ConditionStatus, bool) {
	for i, condition := range status.Conditions {
		if condition.Type == conditionType {
			return &status.Conditions[i], true
		}
	}
	return nil, false
}

func (status *ZoneStatus) EmptyConditions() bool {
	return len(status.Conditions) == 0
}
