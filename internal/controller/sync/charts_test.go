package sync

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"hiro.io/anyapplication/internal/controller/types"
	"hiro.io/anyapplication/internal/helm"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Charts", func() {
	var (
		helmClient helm.HelmClient
		charts     types.Charts
	)

	BeforeEach(func() {
		config := &rest.Config{
			Host: "https://test",
		}
		options := helm.HelmClientOptions{
			RestConfig: config,
			KubeVersion: &chartutil.KubeVersion{
				Version: fmt.Sprintf("v%s.%s.0", "1", "23"),
				Major:   "1",
				Minor:   "23",
			},
		}
		helmClient, _ = helm.NewHelmClient(&options)

		charts = NewCharts(context.TODO(), helmClient, &ChartsOptions{
			SyncPeriod: 100 * time.Millisecond,
		}, logf.Log)

	})

	It("should sync chart for a specific version", func() {
		version, err := types.NewChartVersion("2.0.1")
		Expect(err).NotTo(HaveOccurred())

		latest, err := charts.AddAndGetLatest("nginx-ingress", "https://helm.nginx.com/stable", version)
		Expect(err).NotTo(HaveOccurred())
		Expect(latest).NotTo(BeNil())
		Expect(latest.ChartId.RepoUrl).To(Equal("https://helm.nginx.com/stable"))
		Expect(latest.ChartId.ChartName).To(Equal("nginx-ingress"))
		Expect(latest.Version.ToString()).To(Equal("2.0.1"))
	})

	It("should sync new version of chart for a version constraint", func() {

		version, err := types.NewChartVersion("^2.x")
		Expect(err).NotTo(HaveOccurred())

		latest, err := charts.AddAndGetLatest("nginx-ingress", "https://helm.nginx.com/stable", version)
		Expect(err).NotTo(HaveOccurred())
		Expect(latest).NotTo(BeNil())
		Expect(latest.ChartId.RepoUrl).To(Equal("https://helm.nginx.com/stable"))
		Expect(latest.ChartId.ChartName).To(Equal("nginx-ingress"))
		Expect(latest.Version.ToString()).To(Equal("2.2.1"))
	})

	It("should render chart", func() {
		version, _ := types.NewChartVersion("2.0.1")

		latest, err := charts.AddAndGetLatest("nginx-ingress", "https://helm.nginx.com/stable", version)
		Expect(err).NotTo(HaveOccurred())

		instance := &types.ApplicationInstance{
			Name:        "test-instance",
			Namespace:   "default",
			ReleaseName: "test-release",
			InstanceId:  "test-instance-id",
			ValuesYaml:  "",
		}
		rendered, err := charts.Render(latest, instance)
		Expect(err).NotTo(HaveOccurred())
		Expect(rendered).NotTo(BeNil())

		Expect(rendered.Instance).To(Equal(*instance))
		Expect(rendered.Key).To(Equal(*latest))
		Expect(rendered.Resources).To(HaveLen(23))
	})

})
