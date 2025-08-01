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
	Source            ApplicationSourceSpec `json:"source" validate:"required"`
	Zones             int                   `json:"zones"`
	SyncPolicy        SyncPolicySpec        `json:"syncPolicy,omitempty"`
	PlacementStrategy PlacementStrategySpec `json:"placementStrategy,omitempty"`
	RecoverStrategy   RecoverStrategySpec   `json:"recoverStrategy,omitempty"`
}

type ApplicationSourceSpec struct {
	HelmSelector *ApplicationSourceHelm `json:"helm,omitempty"`
}

type ApplicationSourceHelm struct {
	Repository  string `json:"repository"`
	ReleaseName string `json:"releaseName,omitempty"`
	Chart       string `json:"chart"`
	Version     string `json:"version"`
	Namespace   string `json:"namespace"`
	Values      string `json:"values,omitempty"`
	// Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation
	Parameters []HelmParameter `json:"parameters,omitempty"`
	// SkipCrds skips custom resource definition installation step (Helm's --skip-crds)
	SkipCrds bool `json:"skipCrds,omitempty"`
}

type HelmParameter struct {
	// Name is the name of the Helm parameter
	Name string `json:"name,omitempty"`

	// Value is the value for the Helm parameter
	Value string `json:"value,omitempty"`

	// ForceString determines whether to tell Helm to interpret booleans and numbers as strings
	ForceString bool `json:"forceString,omitempty"`
}

type SyncPolicySpec struct {
	// Automated will keep an application synced to the target revision
	Automated *SyncPolicyAutomated `json:"automated,omitempty"`

	// Options allow you to specify whole app sync-options
	SyncOptions *[]string `json:"syncOptions,omitempty"`

	// Retry controls failed sync retry behavior
	Retry *RetryStrategy `json:"retry,omitempty"`
}

type RetryStrategy struct {
	// Limit is the maximum number of attempts for retrying a failed sync. If set to 0, no retries will be performed.
	Limit int64 `json:"limit,omitempty"`

	// Backoff controls how to backoff on subsequent retries of failed syncs
	Backoff *Backoff `json:"backoff,omitempty"`
}

type Backoff struct {
	// Duration is the amount to back off. Default unit is seconds, but could also be a duration (e.g. "2m", "1h")
	Duration string `json:"duration,omitempty"`

	// Factor is a factor to multiply the base duration after each failed retry
	Factor *int64 `json:"factor,omitempty"`

	// MaxDuration is the maximum amount of time allowed for the backoff strategy
	MaxDuration *string `json:"maxDuration,omitempty"`
}

type SyncPolicyAutomated struct {
	// Prune specifies whether to delete resources from the cluster that are not found in the sources anymore as part of automated sync (default: false)
	Prune bool `json:"prune,omitempty"`

	// SelfHeal specifes whether to revert resources back to their desired state upon modification in the cluster (default: false)
	SelfHeal bool `json:"selfHeal,omitempty"`

	// AllowEmpty allows apps have zero live resources (default: false)
	AllowEmpty bool `json:"allowEmpty,omitempty"`
}

type PlacementStrategySpec struct {
	Strategy PlacementStrategy `json:"strategy"`
}

type RecoverStrategySpec struct {
	Tolerance  int `json:"tolerance,omitempty"`
	MaxRetries int `json:"maxRetries,omitempty"`
}

// AnyApplicationStatus defines the observed state of AnyApplication.
type AnyApplicationStatus struct {
	Ownership OwnershipStatus `json:"ownership"`
	Zones     []ZoneStatus    `json:"zones,omitempty"`
}

type OwnershipStatus struct {
	Epoch      int64       `json:"epoch"`
	State      GlobalState `json:"state"`
	Owner      string      `json:"owner"`
	Placements []Placement `json:"placements,omitempty"`
}

type ZoneStatus struct {
	ZoneId       string            `json:"zoneId"`
	ZoneVersion  int64             `json:"version"`
	ChartVersion string            `json:"chartVersion,omitempty"`
	Conditions   []ConditionStatus `json:"conditions,omitempty"`
}

type Placement struct {
	Zone         string   `json:"zone"`
	NodeAffinity []string `json:"nodeAffinity,omitempty"`
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
