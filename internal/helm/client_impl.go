package helm

import (
	"time"

	"net/url"
	"strings"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/mittwald/go-helm-client/values"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/rest"
)

type HelmClientOptions struct {
	KubernetesHost string
	Debug          bool
	Linting        bool
	KubeVersion    *chartutil.KubeVersion
	UpgradeCRDs    bool
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
			Linting:   options.Linting,
		},
		RestConfig: &rest.Config{
			Host: options.KubernetesHost,
		},
	}
	client, err := helmclient.NewClientFromRestConf(&opts)

	return &HelmClientImpl{client, options}, err
}

type TemplateArgs struct {
	releaseName   string
	repoUrl       string
	chartName     string
	namespace     string
	version       string
	ValuesOptions values.Options
	labels        map[string]string
}

func (h *HelmClientImpl) Template(args *TemplateArgs) (string, error) {

	repoName, err := DeriveUniqueHelmRepoName(args.repoUrl)
	if err != nil {
		return "", err
	}

	// Define a private chart repository
	chartRepo := repo.Entry{
		Name:               repoName,
		URL:                args.repoUrl,
		PassCredentialsAll: false,
	}

	// Add a chart-repository to the client.
	if err := h.client.AddOrUpdateChartRepo(chartRepo); err != nil {
		panic(err)
	}

	chartSpec := helmclient.ChartSpec{
		ReleaseName:   args.releaseName,
		ChartName:     repoName + "/" + args.chartName,
		Version:       args.version,
		Namespace:     args.namespace,
		UpgradeCRDs:   h.options.UpgradeCRDs,
		Wait:          true,
		Timeout:       32 * time.Second,
		ValuesOptions: args.ValuesOptions,
		Labels:        args.labels,
	}

	options := &helmclient.HelmTemplateOptions{
		KubeVersion: h.options.KubeVersion,
		APIVersions: []string{},
	}

	chartBytes, err := h.client.TemplateChart(&chartSpec, options)
	if err != nil {
		panic(err)
	}
	return string(chartBytes), nil
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
