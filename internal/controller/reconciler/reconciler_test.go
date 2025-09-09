// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package reconciler

// import (
// 	"testing"

// 	ginkgo "github.com/onsi/ginkgo/v2"
// 	"github.com/onsi/gomega"
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// )

// func TestReconilerSuite(t *testing.T) {
// 	gomega.RegisterFailHandler(ginkgo.Fail)
// 	ginkgo.RunSpecs(t, "Reconciler Suite")
// }

// var _ = Describe("Reconciler", func() {
// 	var (
// 	// fakeClock     clock.Clock
// 	// runtimeConfig config.ApplicationRuntimeConfig
// 	// jobFactory       job.AsyncJobFactoryImpl
// 	// application      v1.AnyApplication
// 	// localApplication mo.Option[local.LocalApplication]
// 	// globalApplication GlobalApplication
// 	)

// 	BeforeEach(func() {
// 		// fakeClock = clock.NewFakeClock()
// 		// runtimeConfig = config.ApplicationRuntimeConfig{
// 		// 	ZoneId: "zone",
// 		// }

// 		// jobFactory = job.NewAsyncJobFactory(&runtimeConfig, fakeClock)
// 		// localApplication = mo.None[local.LocalApplication]()

// 		// application = v1.AnyApplication{
// 		// 	ObjectMeta: metav1.ObjectMeta{
// 		// 		Name:      "test-app",
// 		// 		Namespace: "default",
// 		// 	},
// 		// 	Spec: v1.AnyApplicationSpec{
// 		// 		Application: v1.ApplicationMatcherSpec{
// 		// 			HelmSelector: &v1.HelmSelectorSpec{
// 		// 				Repository: "test-repo",
// 		// 				Chart:      "test-chart",
// 		// 				Version:    "1.0.0",
// 		// 			},
// 		// 		},
// 		// 		Zones: 1,
// 		// 		PlacementStrategy: v1.PlacementStrategySpec{
// 		// 			Strategy: v1.PlacementStrategyLocal,
// 		// 		},
// 		// 		RecoverStrategy: v1.RecoverStrategySpec{},
// 		// 	},
// 		// 	Status: v1.AnyApplicationStatus{
// 		// 		Owner: "otherzone",
// 		// 		State: v1.UnknownGlobalState,
// 		// 	},
// 		// }
// 		// globalApplication = NewFromLocalApplication(localApplication, fakeClock, &application, &runtimeConfig)

// 	})

// 	It("should not react on new application", func() {

// 		// statusResult := globalApplication.DeriveNewStatus(EmptyJobConditions(), jobFactory)

// 		// status := statusResult.Status.OrEmpty()
// 		// jobs := statusResult.Jobs
// 		// Expect(status).To(Equal(v1.AnyApplicationStatus{}))

// 		// Expect(jobs.JobsToAdd).To(Equal(mo.None[job.AsyncJob]()))
// 		// Expect(jobs.JobsToRemove).To(Equal(mo.None[job.AsyncJobType]()))
// 	})

// })
