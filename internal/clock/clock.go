package clock

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClockImpl struct{}

func NewClock() Clock {
	return &ClockImpl{}
}

func (c *ClockImpl) NowTime() metav1.Time {
	return metav1.Now()
}

type FakeClock struct {
	nowMillis int64
}

func NewFakeClock() *FakeClock {
	return &FakeClock{}
}

func (c *FakeClock) NowTime() metav1.Time {
	t := time.UnixMilli(c.nowMillis)
	return metav1.NewTime(t)
}

func (c *FakeClock) SetNow(nowMillis int64) {
	c.nowMillis = nowMillis
}
