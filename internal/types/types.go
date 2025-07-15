package types

import "context"

type ApplicationReports interface {
	Fetch(ctx context.Context, instanceId string, namespace string) (*ApplicationReport, error)
}
