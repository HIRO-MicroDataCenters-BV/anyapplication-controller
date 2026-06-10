/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dcpv1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/controller"
)

var _ = Describe("AnyApplication Webhook", func() {
	var (
		obj       *dcpv1.AnyApplication
		oldObj    *dcpv1.AnyApplication
		validator AnyApplicationCustomValidator
		defaulter AnyApplicationCustomDefaulter
	)

	BeforeEach(func() {
		obj = &dcpv1.AnyApplication{}
		oldObj = &dcpv1.AnyApplication{}
		validator = AnyApplicationCustomValidator{currentZoneId: "zoneA"}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = AnyApplicationCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
	})

	AfterEach(func() {
	})

	Context("When creating AnyApplication under Defaulting Webhook", func() {
		It("Should add finalizer when needed", func() {
			By("simulating a scenario where defaults should be applied")
			obj.Finalizers = nil
			By("calling the Default method to apply defaults")
			Expect(defaulter.Default(ctx, obj)).Error().To(Not(HaveOccurred()))
			By("checking that the default values are set")
			Expect(obj.Finalizers).To(Equal([]string{controller.AnyApplicationFinalizerName}))
		})
	})

	Context("When creating or updating AnyApplication under Validating Webhook", func() {
		It("Should allow removal if current zone is owning zone", func() {
			By("simulating an valid delete scenario")
			obj.Status.Ownership.State = dcpv1.NewGlobalState
			obj.Status.Ownership.Owner = "zoneA"
			Expect(validator.ValidateDelete(ctx, obj)).Error().To(Not(HaveOccurred()))
		})

		It("Should deny removal if current zone is non owning zone", func() {
			By("simulating an invalid delete scenario")
			obj.Status.Ownership.State = dcpv1.NewGlobalState
			obj.Status.Ownership.Owner = "zoneB"
			Expect(validator.ValidateDelete(ctx, obj)).Error().To(HaveOccurred())
		})

		It("Should allow removal if current zone is not owning zone and state is Removing", func() {
			By("simulating an valid delete scenario")
			obj.Status.Ownership.State = dcpv1.RemovingGlobalState
			obj.Status.Ownership.Owner = "zoneB"
			Expect(validator.ValidateDelete(ctx, obj)).Error().To(Not(HaveOccurred()))
		})

	})

})
