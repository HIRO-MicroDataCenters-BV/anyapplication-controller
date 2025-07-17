package status

import (
	"context"
	"fmt"
	"sync/atomic"

	v1 "hiro.io/anyapplication/api/v1"
	"hiro.io/anyapplication/internal/clock"
	"hiro.io/anyapplication/internal/controller/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStatusUpdate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Status Update Suite")
}

var _ = Describe("AddOrUpdateStatusCondition", func() {
	var (
		ctx         context.Context
		fakeClient  client.Client
		application *v1.AnyApplication
		scheme      *runtime.Scheme
		fakeClock   clock.Clock
		log         logr.Logger
		fakeEvents  events.Events
	)

	BeforeEach(func() {
		ctx = context.TODO()
		fakeClock = clock.NewFakeClock()
		scheme = runtime.NewScheme()
		fakeEvents = events.NewFakeEvents()

		_ = v1.AddToScheme(scheme)

		application = &v1.AnyApplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: v1.AnyApplicationSpec{
				Source: v1.ApplicationSourceSpec{
					HelmSelector: &v1.ApplicationSourceHelm{
						Repository: "test-repo",
						Chart:      "test-chart",
						Version:    "1.0.0",
					},
				},
				Zones: 1,
				PlacementStrategy: v1.PlacementStrategySpec{
					Strategy: v1.PlacementStrategyLocal,
				},
				RecoverStrategy: v1.RecoverStrategySpec{},
			},
			Status: v1.AnyApplicationStatus{
				Owner: "zone",
				State: v1.PlacementGlobalState,
				Zones: []v1.ZoneStatus{
					{
						ZoneId:      "zone",
						ZoneVersion: 0,
						Conditions: []v1.ConditionStatus{
							{
								Type:               v1.LocalConditionType,
								ZoneId:             "zone",
								Status:             string(v1.PlacementStatusInProgress),
								LastTransitionTime: fakeClock.NowTime(),
							},
						},
					},
				},
			},
		}

		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(application).
			WithStatusSubresource(&v1.AnyApplication{}).
			Build()
		log = logf.Log.WithName("UndeployJob")
	})

	It("should add a new zone and condition if zone does not exist", func() {
		newCondition := v1.ConditionStatus{
			Type:   v1.PlacementConditionType,
			ZoneId: "zone2",
			Status: string(v1.PlacementStatusDone),
		}
		oldCondition := v1.ConditionStatus{
			Type:               v1.LocalConditionType,
			ZoneId:             "zone",
			Status:             string(v1.PlacementStatusInProgress),
			LastTransitionTime: fakeClock.NowTime(),
		}
		stopRetrying := atomic.Bool{}
		statusUpdater := NewStatusUpdater(ctx,
			log, fakeClient, client.ObjectKeyFromObject(application), "zone2", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, newCondition)

		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(oldCondition))

		zone2Status, _ := updatedApp.Status.GetStatusFor("zone2")
		Expect(zone2Status.Conditions).To(ContainElement(newCondition))
	})

	It("should add a new condition if it does not exist to existing zone", func() {
		newCondition := v1.ConditionStatus{
			Type:   v1.PlacementConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusDone),
		}
		stopRetrying := atomic.Bool{}
		statusUpdater := NewStatusUpdater(ctx, log, fakeClient, client.ObjectKeyFromObject(application), "zone", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, newCondition)

		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(newCondition))
	})

	It("should update an existing condition if it exists", func() {
		updatedCondition := v1.ConditionStatus{
			Type:   v1.LocalConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusFailure),
		}
		stopRetrying := atomic.Bool{}
		statusUpdater := NewStatusUpdater(ctx, log, fakeClient, client.ObjectKeyFromObject(application), "zone", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, updatedCondition)

		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(updatedCondition))
	})

	It("should not update if the condition is unchanged", func() {
		existingCondition := *application.Status.GetOrCreateStatusFor("zone").Conditions[0].DeepCopy()

		stopRetrying := atomic.Bool{}

		statusUpdater := NewStatusUpdater(ctx, log, fakeClient, client.ObjectKeyFromObject(application), "zone", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, existingCondition)
		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(existingCondition))
	})

	It("should remove a specified condition from a zone", func() {
		// Add two conditions to the zone
		condToKeep := v1.ConditionStatus{
			Type:   v1.PlacementConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusDone),
		}
		condToRemove := v1.ConditionStatus{
			Type:   v1.LocalConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusInProgress),
		}
		application.Status.GetOrCreateStatusFor("zone").Conditions = []v1.ConditionStatus{condToKeep, condToRemove}
		Expect(fakeClient.Status().Update(ctx, application)).To(Succeed())

		stopRetrying := atomic.Bool{}
		statusUpdater := NewStatusUpdater(ctx, log, fakeClient, client.ObjectKeyFromObject(application), "zone", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, condToKeep, v1.LocalConditionType)
		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(condToKeep))
		for _, c := range zoneStatus.Conditions {
			Expect(c.Type).NotTo(Equal(v1.LocalConditionType))
		}
	})

	It("should remove multiple specified conditions from a zone", func() {
		cond1 := v1.ConditionStatus{
			Type:   v1.PlacementConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusDone),
		}
		cond2 := v1.ConditionStatus{
			Type:   v1.LocalConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusInProgress),
		}
		cond3 := v1.ConditionStatus{
			Type:   v1.DeploymenConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusInProgress),
		}
		application.Status.GetOrCreateStatusFor("zone").Conditions = []v1.ConditionStatus{cond1, cond2, cond3}
		Expect(fakeClient.Status().Update(ctx, application)).To(Succeed())

		stopRetrying := atomic.Bool{}
		statusUpdater := NewStatusUpdater(ctx, log, fakeClient, client.ObjectKeyFromObject(application), "zone", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, cond1, v1.LocalConditionType, v1.DeploymenConditionType)
		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		fmt.Printf("%v", updatedApp)

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(cond1))
		for _, c := range zoneStatus.Conditions {
			Expect(c.Type).NotTo(Equal(v1.LocalConditionType))
			Expect(c.Type).NotTo(Equal(v1.DeploymenConditionType))
		}
	})

	It("should do nothing if conditionsToRemove are not present", func() {
		cond := v1.ConditionStatus{
			Type:   v1.PlacementConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusDone),
		}
		application.Status.GetOrCreateStatusFor("zone").Conditions = []v1.ConditionStatus{cond}
		Expect(fakeClient.Status().Update(ctx, application)).To(Succeed())

		stopRetrying := atomic.Bool{}
		statusUpdater := NewStatusUpdater(ctx, log, fakeClient, client.ObjectKeyFromObject(application), "zone", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, cond, v1.LocalConditionType)
		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(cond))
		Expect(zoneStatus.Conditions).To(HaveLen(1))
	})

	It("should add a new condition and remove an old one in a single call", func() {
		oldCond := v1.ConditionStatus{
			Type:   v1.LocalConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusInProgress),
		}
		newCond := v1.ConditionStatus{
			Type:   v1.PlacementConditionType,
			ZoneId: "zone",
			Status: string(v1.PlacementStatusDone),
		}
		application.Status.GetOrCreateStatusFor("zone").Conditions = []v1.ConditionStatus{oldCond}
		Expect(fakeClient.Status().Update(ctx, application)).To(Succeed())

		stopRetrying := atomic.Bool{}
		statusUpdater := NewStatusUpdater(ctx, log, fakeClient, client.ObjectKeyFromObject(application), "zone", &fakeEvents)
		event := events.Event{}
		err := statusUpdater.UpdateCondition(&stopRetrying, event, newCond, v1.LocalConditionType)
		Expect(err).ToNot(HaveOccurred())

		updatedApp := &v1.AnyApplication{}
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(application), updatedApp)
		Expect(err).ToNot(HaveOccurred())

		zoneStatus, _ := updatedApp.Status.GetStatusFor("zone")
		Expect(zoneStatus.Conditions).To(ContainElement(newCond))
		for _, c := range zoneStatus.Conditions {
			Expect(c.Type).NotTo(Equal(v1.LocalConditionType))
		}
	})

})
