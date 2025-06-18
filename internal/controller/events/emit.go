package events

import (
	dcpv1 "hiro.io/anyapplication/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

type Events struct {
	recorder record.EventRecorder
}

func NewEvents(recorder record.EventRecorder) Events {
	return Events{
		recorder: recorder,
	}
}

func (e *Events) Emit(application *dcpv1.AnyApplication, event Event) {
	e.recorder.Event(application, corev1.EventTypeNormal, event.Reason, event.Msg)
}

func NewFakeEvents() Events {
	return Events{
		recorder: NewFakeEventRecorder(),
	}
}
