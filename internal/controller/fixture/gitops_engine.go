package fixture

import (
	"context"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type FakeGitOpsEngine struct {
	result []common.ResourceSyncResult
}

func NewFakeGitopsEngine() *FakeGitOpsEngine {
	return &FakeGitOpsEngine{}
}

func (f *FakeGitOpsEngine) Run() (engine.StopFunc, error) {
	return func() {}, nil
}

func (f *FakeGitOpsEngine) MockSyncResult(result []common.ResourceSyncResult) {
	f.result = result
}

func (f *FakeGitOpsEngine) Sync(
	ctx context.Context,
	resources []*unstructured.Unstructured,
	isManaged func(r *cache.Resource) bool,
	revision string,
	namespace string,
	opts ...sync.SyncOpt,
) ([]common.ResourceSyncResult, error) {
	return f.result, nil
}
