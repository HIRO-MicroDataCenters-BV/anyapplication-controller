package helm

type HelmClient interface {
	AddOrUpdateChartRepo(repoURL string) (string, error)
	Template(args *TemplateArgs) (string, error)
}
