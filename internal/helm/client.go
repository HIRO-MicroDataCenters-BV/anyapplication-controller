package helm

type HelmClient interface {
	Template(args *TemplateArgs) (string, error)
}
