package helm

import (
	"time"

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

type HelmClient struct {
	client  helmclient.Client
	options *HelmClientOptions
}

func NewHelmClient(options *HelmClientOptions) (*HelmClient, error) {
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

	return &HelmClient{client, options}, err
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

func (h *HelmClient) Template(args *TemplateArgs) (string, error) {
	// Define a private chart repository
	chartRepo := repo.Entry{
		Name:               "nginx-repo",
		URL:                args.repoUrl,
		PassCredentialsAll: false,
	}

	// Add a chart-repository to the client.
	if err := h.client.AddOrUpdateChartRepo(chartRepo); err != nil {
		panic(err)
	}

	chartSpec := helmclient.ChartSpec{
		ReleaseName: args.releaseName,
		ChartName:   "nginx-repo/" + args.chartName,
		Version:     args.version,
		Namespace:   args.namespace,
		UpgradeCRDs: h.options.UpgradeCRDs,
		Wait:        true,
		Timeout:     32 * time.Second,
		ValuesYaml:  ``,
		Labels:      args.labels,
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
