// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("AddLabels", func() {
	var (
		log logr.Logger
	)

	BeforeEach(func() {
		log = logr.Discard()
	})

	It("should add labels if metadata does not exist", func() {
		obj := unstructured.Unstructured{}
		obj.SetKind("ConfigMap")
		obj.SetName("test-cm")
		obj.SetNamespace("default")
		// No labels set

		labels := map[string]string{
			"foo": "bar",
		}

		result := AddLabels(labels, log)(obj)
		Expect(result.GetLabels()).To(HaveKeyWithValue("foo", "bar"))
	})

	It("should merge new labels with existing labels", func() {
		obj := unstructured.Unstructured{}
		obj.SetKind("ConfigMap")
		obj.SetName("test-cm")
		obj.SetNamespace("default")
		obj.SetLabels(map[string]string{
			"existing": "label",
		})

		labels := map[string]string{
			"foo": "bar",
		}

		result := AddLabels(labels, log)(obj)
		Expect(result.GetLabels()).To(HaveKeyWithValue("foo", "bar"))
		Expect(result.GetLabels()).To(HaveKeyWithValue("existing", "label"))
	})

	It("should add labels to spec.template.metadata.labels for Deployment", func() {
		obj := unstructured.Unstructured{}
		obj.Object = map[string]interface{}{
			"kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deploy",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{},
				},
			},
		}

		newLabels := map[string]string{
			"foo": "bar",
		}

		result := AddLabels(newLabels, log)(obj)
		labelsMap, found, err := unstructured.NestedStringMap(result.Object, "spec", "template", "metadata", "labels")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(labelsMap).To(HaveKeyWithValue("foo", "bar"))
	})

	It("should add labels to spec.template.metadata.labels for Deployment if the metadata section does not exist", func() {
		obj := unstructured.Unstructured{}
		obj.Object = map[string]interface{}{
			"kind": "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deploy",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{},
			},
		}

		newLabels := map[string]string{
			"foo": "bar",
		}

		result := AddLabels(newLabels, log)(obj)
		labelsMap, found, err := unstructured.NestedStringMap(result.Object, "spec", "template", "metadata", "labels")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(labelsMap).To(HaveKeyWithValue("foo", "bar"))
	})

	It("should merge labels into spec.template.metadata.labels for StatefulSet", func() {
		obj := unstructured.Unstructured{}
		obj.Object = map[string]interface{}{
			"kind": "StatefulSet",
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"existing": "label",
						},
					},
				},
			},
		}
		obj.SetName("test-ss")
		obj.SetNamespace("default")

		newLabels := map[string]string{
			"foo": "bar",
		}

		result := AddLabels(newLabels, log)(obj)
		labelsMap, found, err := unstructured.NestedStringMap(result.Object, "spec", "template", "metadata", "labels")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(labelsMap).To(HaveKeyWithValue("foo", "bar"))
		Expect(labelsMap).To(HaveKeyWithValue("existing", "label"))
	})

})
