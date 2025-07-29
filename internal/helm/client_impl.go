package helm

import (
	"fmt"
	"time"

	"net/url"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	helmclient "github.com/mittwald/go-helm-client"
	"github.com/mittwald/go-helm-client/values"
	"sigs.k8s.io/yaml"

	"github.com/cockroachdb/errors"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

type HelmClientOptions struct {
	RestConfig  *rest.Config
	Debug       bool
	Linting     bool
	KubeVersion *chartutil.KubeVersion
}

type HelmClientImpl struct {
	client  helmclient.Client
	options *HelmClientOptions
}

func NewHelmClient(options *HelmClientOptions) (*HelmClientImpl, error) {
	opts := helmclient.RestConfClientOptions{
		Options: &helmclient.Options{
			Namespace: "default",
			Debug:     options.Debug,
			DebugLog:  func(format string, v ...interface{}) {},
			Linting:   options.Linting,
		},
		RestConfig: options.RestConfig,
	}
	client, err := helmclient.NewClientFromRestConf(&opts)

	return &HelmClientImpl{client, options}, err
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

	indexFile, err := chartRepo.DownloadIndexFile()
	if err != nil {
		return nil, fmt.Errorf("looks like %q is not a valid chart repository or cannot be reached: %w", repoURL, err)
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
	return AddLabelsToManifest(manifest, args.Labels)
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

// Go Helm Client does not support extra labels
// This is post processing step to fix that
func AddLabelsToManifest(manifest string, newLabels map[string]string) (string, error) {
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
		// Merge new labels into existing ones
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		for k, v := range newLabels {
			labels[k] = v
		}
		obj.SetLabels(labels)

		if obj.GetKind() == "Deployment" || obj.GetKind() == "StatefulSet" || obj.GetKind() == "DaemonSet" {
			// add labels to the spec template
			spec, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "metadata")
			if err != nil {
				return "", errors.Wrap(err, "Failed to get spec template metadata")
			}
			if found {
				if spec["labels"] == nil {
					spec["labels"] = make(map[string]interface{})
				}
				for k, v := range newLabels {
					spec["labels"].(map[string]interface{})[k] = v
				}
				if err := unstructured.SetNestedMap(obj.Object, spec, "spec", "template", "metadata"); err != nil {
					return "", errors.Wrap(err, "Failed to set spec template metadata")
				}
			}
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

// func (c *helmclient.HelmClient) AddOrUpdateChartRepo(entry repo.Entry) error {
// 	chartRepo, err := repo.NewChartRepository(&entry, c.Providers)
// 	if err != nil {
// 		return err
// 	}

// 	chartRepo.CachePath = c.Settings.RepositoryCache

// 	if c.storage.Has(entry.Name) {
// 		c.DebugLog("WARNING: repository name %q already exists", entry.Name)
// 		return nil
// 	}

// 	if !registry.IsOCI(entry.URL) {
// 		_, err = chartRepo.DownloadIndexFile()
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	c.storage.Update(&entry)
// 	err = c.storage.WriteFile(c.Settings.RepositoryConfig, 0o644)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
