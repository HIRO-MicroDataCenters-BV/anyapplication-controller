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

	It("Sync chart", func() {
		version, err := types.NewChartVersion("2.0.1")
		Expect(err).NotTo(HaveOccurred())

		version2, err2 := types.NewChartVersion("^2.x")
		Expect(err2).NotTo(HaveOccurred())

		fmt.Printf("Version: %s, Version2: %s\n", version.ToString(), version2.ToString())

		charts.AddChart("nginx-ingress", "https://helm.nginx.com/stable", version)
	})

	It("Sync new version of chart", func() {
	})

})
