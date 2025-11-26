// SPDX-FileCopyrightText: 2025 HIRO affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"fmt"
	"os"
	"time"

	"net/url"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/cockroachdb/errors"
	"github.com/go-logr/logr"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/mittwald/go-helm-client/values"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

const (
	defaultCachePath            = "/tmp/.helmcache"
	defaultRepositoryConfigPath = "/tmp/.helmrepo"
)

type HelmClientOptions struct {
	RestConfig           *rest.Config
	Debug                bool
	Linting              bool
	KubeVersion          *chartutil.KubeVersion
	ClientId             string
	Log                  logr.Logger
	buildClusterScopeMap func(cfg *rest.Config) (map[schema.GroupVersionKind]bool, error)
}

type HelmClientImpl struct {
	client  helmclient.Client
	options *HelmClientOptions
}

func NewHelmClient(options *HelmClientOptions) (*HelmClientImpl, error) {
	options.buildClusterScopeMap = buildGVKClusterScopeMap
	if options.ClientId == "" {
		clientId, err := RandClient()
		if err != nil {
			return nil, err
		}
		options.ClientId = clientId
	}

	opts := helmclient.RestConfClientOptions{
		Options: &helmclient.Options{
			Debug:            options.Debug,
			DebugLog:         func(format string, v ...interface{}) {},
			Linting:          options.Linting,
			RepositoryConfig: fmt.Sprintf("%s-%s", defaultRepositoryConfigPath, options.ClientId),
			RepositoryCache:  fmt.Sprintf("%s-%s", defaultCachePath, options.ClientId),
		},
		RestConfig: options.RestConfig,
	}
	client, err := helmclient.NewClientFromRestConf(&opts)

	return &HelmClientImpl{client, options}, err
}

func NewTestClient(options *HelmClientOptions) (*HelmClientImpl, error) {
	client, err := NewHelmClient(options)
	options.buildClusterScopeMap = BuildStaticGVKClusterScopeMapForTests
	return client, err
}

type TemplateArgs struct {
	ReleaseName   string
	RepoUrl       string
	ChartName     string
	Namespace     string
	Version       string
	ValuesOptions values.Options
	ValuesYaml    string
	Labels        map[string]string
	UpgradeCRDs   bool
}

func (h *HelmClientImpl) AddOrUpdateChartRepo(repoURL string) (string, error) {

	chartRepo, err := h.addOrUpdateChartRepo(repoURL)
	if err != nil {
		return "", err
	}
	return chartRepo.Name, nil
}

func (h *HelmClientImpl) addOrUpdateChartRepo(repoURL string) (*repo.Entry, error) {
	repoName, err := DeriveUniqueHelmRepoName(repoURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to derive unique helm repo name")
	}

	// Define a private chart repository
	chartRepo := repo.Entry{
		Name:               repoName,
		URL:                repoURL,
		PassCredentialsAll: false,
	}

	// Add a chart-repository to the client.
	if err := h.client.AddOrUpdateChartRepo(chartRepo); err != nil {
		return nil, errors.Wrap(err, "Failed to add or update chart repo")
	}
	return &chartRepo, nil
}

func (h *HelmClientImpl) SyncRepositories() error {
	return h.client.UpdateChartRepos()
}

func (h *HelmClientImpl) FetchVersions(repoURL string, chartName string) ([]*semver.Version, error) {
	if repoURL == "" {
		return nil, errors.New("repoURL cannot be empty")
	}

	entry, err := h.addOrUpdateChartRepo(repoURL)
	if err != nil {
		return nil, err
	}

	chartRepo, err := repo.NewChartRepository(entry, h.client.GetProviders())
	if err != nil {
		return nil, err
	}
	chartRepo.CachePath = h.client.GetSettings().RepositoryCache

	indexFile, err := chartRepo.DownloadIndexFile()
	if err != nil {
		return nil, fmt.Errorf("looks like %q is not a valid chart repository or cannot be reached: %w", repoURL, err)
	}
	attempts := 10
	for attempts > 0 {
		data, err := os.ReadFile(indexFile)
		if err == nil && len(data) != 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
		attempts--
	}

	repoIndex, err := repo.LoadIndexFile(indexFile)
	if err != nil {
		return nil, err
	}

	if entries, ok := repoIndex.Entries[chartName]; ok {
		versions := make([]*semver.Version, 0, len(entries))
		for _, entry := range entries {
			if entry.Version != "" {
				version, err := semver.NewVersion(entry.Version)
				if err != nil {
					fmt.Printf("Failed to parse version %s for chart %s in repository %s: %v", entry.Version, chartName, repoURL, err)
					continue
				}
				versions = append(versions, version)
			}
		}
		return versions, nil
	}

	return nil, nil
}

func (h *HelmClientImpl) Template(args *TemplateArgs) (string, error) {

	repoName, err := h.AddOrUpdateChartRepo(args.RepoUrl)
	if err != nil {
		return "", err
	}
	chartSpec := helmclient.ChartSpec{
		ReleaseName: args.ReleaseName,
		ChartName:   repoName + "/" + args.ChartName,
		Version:     args.Version,
		Namespace:   args.Namespace,
		UpgradeCRDs: args.UpgradeCRDs,
		Wait:        true,
		Timeout:     32 * time.Second,
		Labels:      args.Labels,
	}

	if args.ValuesYaml != "" {
		chartSpec.ValuesYaml = args.ValuesYaml
		chartSpec.ValuesOptions = values.Options{}
	} else {
		chartSpec.ValuesYaml = ""
		chartSpec.ValuesOptions = args.ValuesOptions
	}

	options := &helmclient.HelmTemplateOptions{
		KubeVersion: h.options.KubeVersion,
		APIVersions: []string{},
	}

	chartBytes, err := h.client.TemplateChart(&chartSpec, options)
	if err != nil {
		return "", errors.Wrap(err, "Failed to template chart")
	}
	manifest := string(chartBytes)

	isClusterScope, err := h.options.buildClusterScopeMap(h.options.RestConfig)
	if err != nil {
		return "", errors.Wrap(err, "Unable to build resource map and discover cluster-wide resources")
	}
	// Go Helm Client does not support extra labels and namespace post processing
	// This is post processing step to fix that
	return PostProcessManifests(
		manifest,
		AddLabels(args.Labels, h.options.Log),
		AddNamespace(args.Namespace, isClusterScope, h.options.Log),
	)
}

func DeriveUniqueHelmRepoName(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}

	domain := strings.ReplaceAll(u.Hostname(), ".", "-")
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	pathPart := parts[len(parts)-1]

	return domain + "-" + pathPart, nil
}

// Go Helm Client does not namespace postprocessing
func AddNamespace(namespace string, isClusterScopeRegistry map[schema.GroupVersionKind]bool, log logr.Logger) func(obj unstructured.Unstructured) unstructured.Unstructured {
	return func(obj unstructured.Unstructured) unstructured.Unstructured {
		var updateNamespace = false
		gvk := obj.GroupVersionKind()
		isClusterScope, exists := isClusterScopeRegistry[gvk]
		if exists && !isClusterScope {
			updateNamespace = true
		}
		if !exists && obj.GetKind() != "CustomResourceDefinition" {
			updateNamespace = true
		}

		if updateNamespace && obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
		}

		return obj
	}
}

func AddLabels(newLabels map[string]string, log logr.Logger) func(obj unstructured.Unstructured) unstructured.Unstructured {
	return func(obj unstructured.Unstructured) unstructured.Unstructured {
		// Merge new labels into existing ones
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		for k, v := range newLabels {
			labels[k] = v
		}
		obj.SetLabels(labels)
		// TODO tests and edge cases (no metadata)
		if obj.GetKind() == "Deployment" || obj.GetKind() == "StatefulSet" || obj.GetKind() == "DaemonSet" || obj.GetKind() == "Job" {
			// add labels to the spec template
			spec, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "metadata")
			if err != nil {
				log.Error(err, "Failed to get spec template metadata",
					"kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
			}
			if found {
				if spec["labels"] == nil {
					spec["labels"] = make(map[string]interface{})
				}
				for k, v := range newLabels {
					spec["labels"].(map[string]interface{})[k] = v
				}
				if err := unstructured.SetNestedMap(obj.Object, spec, "spec", "template", "metadata"); err != nil {
					log.Error(err, "Failed to set spec template metadata",
						"kind", obj.GetKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
				}
			}
		}

		return obj
	}
}

// This is post processing step to fix custom labels and namespaces
func PostProcessManifests(manifest string, funcs ...func(obj unstructured.Unstructured) unstructured.Unstructured) (string, error) {
	docs := strings.Split(manifest, "---")
	output := make([]string, 0, 10)

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			// Keep unparseable docs untouched (e.g. comments or notes)
			output = append(output, doc)
			continue
		}
		apiVersion := obj.GetAPIVersion()
		kind := obj.GetKind()
		if apiVersion == "" && kind == "" {
			output = append(output, doc)
			continue
		}

		for _, postprocessor := range funcs {
			obj = postprocessor(obj)
		}

		// Marshal back to YAML
		modifiedYAML, err := yaml.Marshal(obj.Object)
		if err != nil {
			return "", err
		}

		output = append(output, string(modifiedYAML))
	}

	return strings.Join(output, "---\n"), nil
}

// BuildGVKClusterScopeMap returns a map where true = cluster-scoped, false = namespaced.
func buildGVKClusterScopeMap(cfg *rest.Config) (map[schema.GroupVersionKind]bool, error) {
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	gr, err := restmapper.GetAPIGroupResources(disc)
	if err != nil {
		return nil, fmt.Errorf("failed to get API group resources: %w", err)
	}

	result := make(map[schema.GroupVersionKind]bool)

	for _, groupResources := range gr {

		// groupResources.VersionedResources is: map[groupVersionString][]APIResource
		for version, resources := range groupResources.VersionedResources {
			gv := schema.GroupVersion{
				Group:   groupResources.Group.Name, // core="" , others correct
				Version: version,
			}
			for _, r := range resources {
				// skip subresources
				if strings.Contains(r.Name, "/") {
					continue
				}
				gvk := gv.WithKind(r.Kind)
				result[gvk] = !r.Namespaced
			}
		}
	}

	return result, nil
}
