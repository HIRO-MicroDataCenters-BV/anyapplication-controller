// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type FakeEventRecorder struct{}

func NewFakeEventRecorder() FakeEventRecorder {
	return FakeEventRecorder{}
}

func (e FakeEventRecorder) Event(
	object runtime.Object, eventtype, reason, message string,
) {
}

func (e FakeEventRecorder) Eventf(
	object runtime.Object, eventtype, reason, messageFmt string, args ...interface{},
) {
}

func (e FakeEventRecorder) AnnotatedEventf(
	object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{},
) {
}
