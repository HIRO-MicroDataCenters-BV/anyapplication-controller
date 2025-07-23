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
		})

	})

	It("should sync chart", func() {
		version, err := types.NewChartVersion("2.0.1")
		Expect(err).NotTo(HaveOccurred())

		latest, err := charts.AddAndGetLatest("nginx-ingress", "https://helm.nginx.com/stable", version)
		Expect(err).NotTo(HaveOccurred())
		Expect(latest).NotTo(BeNil())
		Expect(latest.ChartId.RepoUrl).To(Equal("https://helm.nginx.com/stable"))
		Expect(latest.ChartId.ChartName).To(Equal("nginx-ingress"))
		Expect(latest.Version.ToString()).To(Equal("2.0.1"))
	})

	It("should render chart", func() {
		version, _ := types.NewChartVersion("2.0.1")

		latest, _ := charts.AddAndGetLatest("nginx-ingress", "https://helm.nginx.com/stable", version)
		instance := &types.ApplicationInstance{
			Name:        "test-instance",
			Namespace:   "default",
			ReleaseName: "test-release",
			ValuesYaml:  "",
		}
		rendered, err := charts.Render(latest, instance)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should sync new version of chart", func() {
		version, err := types.NewChartVersion("2.0.0")
		Expect(err).NotTo(HaveOccurred())

		versionRange, err2 := types.NewChartVersion("^2.x")
		Expect(err2).NotTo(HaveOccurred())

		latest, err := charts.AddAndGetLatest("nginx-ingress", "https://helm.nginx.com/stable", version)
		Expect(err).NotTo(HaveOccurred())
		Expect(latest).NotTo(BeNil())
		Expect(latest.ChartId.RepoUrl).To(Equal("https://helm.nginx.com/stable"))
		Expect(latest.ChartId.ChartName).To(Equal("nginx-ingress"))
		Expect(latest.Version.ToString()).To(Equal("2.0.0"))

		newLatest, err := charts.AddAndGetLatest("nginx-ingress", "https://helm.nginx.com/stable", versionRange)
		Expect(err).NotTo(HaveOccurred())
		Expect(newLatest).NotTo(BeNil())
		Expect(newLatest.ChartId.RepoUrl).To(Equal("https://helm.nginx.com/stable"))
		Expect(newLatest.ChartId.ChartName).To(Equal("nginx-ingress"))
		Expect(newLatest.Version.ToString()).To(Equal("2.0.1"))
	})

})
