// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package fixture

import (
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
	"k8s.io/kubectl/pkg/scheme"
)

func NewTestClusterCacheWithOptions(opts []cache.UpdateSettingsFunc, objs ...runtime.Object) (cache.ClusterCache, *k8sfake.FakeDynamicClient) {
	k8sClient := k8sfake.NewSimpleDynamicClient(scheme.Scheme, objs...)
	reactor := k8sClient.ReactionChain[0]
	k8sClient.PrependReactor("list", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		handled, ret, err = reactor.React(action)
		if err != nil || !handled {
			return
		}
		// make sure list response have resource version
		ret.(metav1.ListInterface).SetResourceVersion("123")
		return
	})

	apiResources := []kube.APIResourceInfo{{
		GroupKind:            schema.GroupKind{Group: "", Kind: "Pod"},
		GroupVersionResource: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "apps", Kind: "ReplicaSet"},
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "apps", Kind: "Deployment"},
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "apps", Kind: "StatefulSet"},
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "apps", Kind: "DaemonSet"},
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}, {
		GroupKind:            schema.GroupKind{Group: "extensions", Kind: "ReplicaSet"},
		GroupVersionResource: schema.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "replicasets"},
		Meta:                 metav1.APIResource{Namespaced: true},
	}}

	opts = append([]cache.UpdateSettingsFunc{
		cache.SetKubectl(&kubetest.MockKubectlCmd{APIResources: apiResources, DynamicClient: k8sClient}),
	}, opts...)

	cache := cache.NewClusterCache(
		&rest.Config{Host: "https://test"},
		opts...,
	)
	return cache, k8sClient
}
