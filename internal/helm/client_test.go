package helm

import (
	"github.com/mittwald/go-helm-client/values"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/chartutil"
	"hiro.io/anyapplication/internal/controller/fixture"
	"k8s.io/client-go/rest"
)

var _ = Describe("HelmClient", func() {
	Context("When reconciling a resource", func() {

		options := &HelmClientOptions{
			RestConfig: &rest.Config{
				Host: "https://localhost:6443",
			},
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
				ReleaseName: "test-release",
				RepoUrl:     "https://helm.nginx.com/stable",
				ChartName:   "nginx-ingress",
				Namespace:   "default",
				Version:     "2.0.1",
				ValuesOptions: values.Options{
					Values: []string{
						"controller.service.type=LoadBalancer",
					},
				},
				Labels: map[string]string{
					"dcp.hiro.io/test": "dcp",
				},
			}

			actual, err := client.Template(&args)

			// fixture.SaveStringFixture("nginx-default.yaml", actual)
			expected := fixture.LoadStringFixture("nginx-default.yaml")

			Expect(actual).To(Equal(expected))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should template helm chart with values yaml", func() {
			args := TemplateArgs{
				ReleaseName: "test-release",
				RepoUrl:     "https://helm.nginx.com/stable",
				ChartName:   "nginx-ingress",
				Namespace:   "default",
				Version:     "2.0.1",
				ValuesYaml: `
controller:
  name: test
  image:
    tag: 1.0.0
  appprotect:
    enable: false
`,
				Labels: map[string]string{
					"dcp.hiro.io/test": "dcp",
				},
			}

			actual, err := client.Template(&args)
			// fixture.SaveStringFixture("nginx-values.yaml", actual)
			expected := fixture.LoadStringFixture("nginx-values.yaml")

			Expect(actual).To(Equal(expected))
			Expect(err).NotTo(HaveOccurred())
		})

	})
})

var _ = Describe("DeriveUniqueHelmRepoName", func() {
	It("should create unique repository name", func() {
		name, err := DeriveUniqueHelmRepoName("https://helm.nginx.com/stable")
		Expect(err).NotTo(HaveOccurred())
		Expect(name).To(Equal("helm-nginx-com-stable"))

	})
})
