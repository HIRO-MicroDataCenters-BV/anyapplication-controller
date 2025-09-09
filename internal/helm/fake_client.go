// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package helm

import semver "github.com/Masterminds/semver/v3"

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

func (c *FakeHelmClient) AddOrUpdateChartRepo(repoURL string) (string, error) {
	return repoURL, nil
}

func (c *FakeHelmClient) SyncRepositories() error { return nil }

func (c *FakeHelmClient) FetchVersions(repoURL string, chartName string) ([]*semver.Version, error) {
	version, err := semver.NewVersion("2.0.1")
	if err != nil {
		return nil, err
	}
	return []*semver.Version{
		version,
	}, nil
}
