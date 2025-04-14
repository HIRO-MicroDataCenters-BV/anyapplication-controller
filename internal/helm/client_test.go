package helm

import (
	"github.com/mittwald/go-helm-client/values"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"hiro.io/anyapplication/internal/controller/fixture"
)

var _ = Describe("HelmClient", func() {
	Context("When reconciling a resource", func() {

		options := &HelmClientOptions{
			KubernetesHost: "https://localhost:6443",
			KubeVersion: &chartutil.KubeVersion{
				Version: "v1.23.10",
				Major:   "1",
				Minor:   "23",
			},
		}
		client, err := NewHelmClient(options)

		if err != nil {
			Fail("Failed to create Helm client")
		}

		It("should template helm chart", func() {
			args := TemplateArgs{
				releaseName: "test-release",
				repoUrl:     "https://helm.nginx.com/stable",
				chartName:   "nginx-ingress",
				namespace:   "default",
				version:     "2.0.1",
				ValuesOptions: values.Options{
					Values: []string{
						"controller.service.type=LoadBalancer",
					},
				},
				labels: map[string]string{
					"dcp.hiro.io/managed-by": "dcp",
				},
			}

			actual, err := client.Template(&args)

			// fixture.SaveStringFixture("nginx.yaml", chartString)
			expected := fixture.LoadStringFixture("nginx.yaml")

			Expect(actual).To(Equal(expected))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
