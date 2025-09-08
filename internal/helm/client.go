// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"crypto/rand"

	semver "github.com/Masterminds/semver/v3"
)

type HelmClient interface {
	AddOrUpdateChartRepo(repoURL string) (string, error)
	SyncRepositories() error
	FetchVersions(repoURL string, chartName string) ([]*semver.Version, error)
	Template(args *TemplateArgs) (string, error)
}

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func RandClient() (string, error) {
	b := make([]byte, 8)

	randomBytes := make([]byte, len(b))
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	for i := range b {
		b[i] = charset[int(randomBytes[i])%len(charset)]
	}
	return string(b), nil
}
