package helm

type FakeHelmClient struct {
}

func NewFakeHelmClient() FakeHelmClient {
	return FakeHelmClient{}
}

func (c FakeHelmClient) Template(args *TemplateArgs) (string, error) {
	return "", nil
}
