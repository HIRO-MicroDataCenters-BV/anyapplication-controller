package clock

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Clock interface {
	NowTime() metav1.Time
}
