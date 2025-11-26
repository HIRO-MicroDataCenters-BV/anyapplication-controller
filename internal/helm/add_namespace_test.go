// SPDX-FileCopyrightText: 2025 HIRO affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("AddNamespace", func() {
	var (
		log logr.Logger
	)

	BeforeEach(func() {
		log = logr.Discard()
	})

	It("should set namespace if resource is namespaced and namespace is empty", func() {
		obj := unstructured.Unstructured{}
		obj.SetKind("ConfigMap")
		obj.SetAPIVersion("v1")
		obj.SetName("test-cm")
		// No namespace set

		isClusterScope := map[schema.GroupVersionKind]bool{
			{Group: "", Version: "v1", Kind: "ConfigMap"}: false,
		}

		result := AddNamespace("test-ns", isClusterScope, log)(obj)
		Expect(result.GetNamespace()).To(Equal("test-ns"))
	})

	It("should not set namespace if resource is cluster-scoped", func() {
		obj := unstructured.Unstructured{}
		obj.SetKind("Namespace")
		obj.SetAPIVersion("v1")
		obj.SetName("test-ns")
		// No namespace set

		isClusterScope := map[schema.GroupVersionKind]bool{
			{Group: "", Version: "v1", Kind: "Namespace"}: true,
		}

		result := AddNamespace("test-ns", isClusterScope, log)(obj)
		Expect(result.GetNamespace()).To(Equal(""))
	})

	It("should not overwrite existing namespace", func() {
		obj := unstructured.Unstructured{}
		obj.SetKind("ConfigMap")
		obj.SetAPIVersion("v1")
		obj.SetName("test-cm")
		obj.SetNamespace("existing-ns")

		isClusterScope := map[schema.GroupVersionKind]bool{
			{Group: "", Version: "v1", Kind: "ConfigMap"}: false,
		}

		result := AddNamespace("test-ns", isClusterScope, log)(obj)
		Expect(result.GetNamespace()).To(Equal("existing-ns"))
	})

	It("should set namespace for unknown GVKs except CRDs", func() {
		obj := unstructured.Unstructured{}
		obj.SetKind("MyCustomResource")
		obj.SetAPIVersion("mygroup/v1")
		obj.SetName("test-cr")
		// No namespace set

		isClusterScope := map[schema.GroupVersionKind]bool{}

		result := AddNamespace("test-ns", isClusterScope, log)(obj)
		Expect(result.GetNamespace()).To(Equal("test-ns"))
	})

	It("should not set namespace for CRDs even if unknown GVK", func() {
		obj := unstructured.Unstructured{}
		obj.SetKind("CustomResourceDefinition")
		obj.SetAPIVersion("apiextensions.k8s.io/v1")
		obj.SetName("test-crd")
		// No namespace set

		isClusterScope := map[schema.GroupVersionKind]bool{}

		result := AddNamespace("test-ns", isClusterScope, log)(obj)
		Expect(result.GetNamespace()).To(Equal(""))
	})
})
