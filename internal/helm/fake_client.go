// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	semver "github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type FakeHelmClient struct {
	template string
}

func NewFakeHelmClient() *FakeHelmClient {
	return &FakeHelmClient{}
}

func (c *FakeHelmClient) MockTemplate(template string) {
	c.template = template
}

func (c FakeHelmClient) Template(args *TemplateArgs) (string, error) {
	return c.template, nil
}

func (c *FakeHelmClient) AddOrUpdateChartRepo(repoURL string) (string, error) {
	return repoURL, nil
}

func (c *FakeHelmClient) SyncRepositories() error { return nil }

func (c *FakeHelmClient) FetchVersions(repoURL string, chartName string) ([]*semver.Version, error) {
	version, err := semver.NewVersion("2.0.1")
	if err != nil {
		return nil, err
	}
	return []*semver.Version{
		version,
	}, nil
}

// BuildGVKClusterScopeMapStatic returns a static map of major Kubernetes GVKs → cluster-scoped (true) or namespaced (false).
// this map is not complete and is used for testing purposes
// the one used in production in client_impl.go
func BuildStaticGVKClusterScopeMapForTests(cfg *rest.Config) (map[schema.GroupVersionKind]bool, error) {
	result := make(map[schema.GroupVersionKind]bool)

	m := map[schema.GroupVersionKind]bool{}

	// --- Core /v1 ---
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}] = true
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}] = true
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolume"}] = true

	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}] = false
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}] = false
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}] = false
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}] = false
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}] = false
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Event"}] = false
	m[schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ServiceAccount"}] = false

	// --- RBAC / Cluster-scoped ---
	m[schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}] = true
	m[schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}] = true

	// --- RBAC / Namespaced ---
	m[schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"}] = false
	m[schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"}] = false

	// --- Apps ---
	m[schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}] = false
	m[schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}] = false
	m[schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}] = false
	m[schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}] = false

	// --- Batch ---
	m[schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}] = false
	m[schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}] = false

	// --- Networking ---
	m[schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"}] = false
	m[schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"}] = false
	m[schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "IngressClass"}] = true // CLUSTER-SCOPED

	// --- Autoscaling ---
	m[schema.GroupVersionKind{Group: "autoscaling", Version: "v1", Kind: "HorizontalPodAutoscaler"}] = false
	m[schema.GroupVersionKind{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"}] = false

	// --- Scheduling ---
	m[schema.GroupVersionKind{Group: "scheduling.k8s.io", Version: "v1", Kind: "PriorityClass"}] = true

	// --- Storage ---
	m[schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}] = true
	m[schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "CSIDriver"}] = true
	m[schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "VolumeAttachment"}] = true
	m[schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "CSIStorageCapacity"}] = false

	// --- API registration ---
	m[schema.GroupVersionKind{Group: "apiregistration.k8s.io", Version: "v1", Kind: "APIService"}] = true

	// --- Admission ---
	m[schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"}] = true
	m[schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"}] = true

	return result, nil
}
