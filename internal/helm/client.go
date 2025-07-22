package helm

type HelmClient interface {
	AddOrUpdateChartRepo(repoURL string) (string, error)
	SyncRepositories()
	Template(args *TemplateArgs) (string, error)
}
