package helm

type FakeHelmClient struct {
	template string
}

func NewFakeHelmClient() *FakeHelmClient {
	return &FakeHelmClient{}
}

func (c *FakeHelmClient) MockTemplate(template string) {
	c.template = template
}

func (c FakeHelmClient) Template(args *TemplateArgs) (string, error) {
	return c.template, nil
}
