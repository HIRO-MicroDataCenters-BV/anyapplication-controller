package fixture

import (
	v1 "hiro.io/anyapplication/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Condition(condType v1.ApplicationConditionType, zoneId string, version string, time metav1.Time, status string) v1.ConditionStatus {
	return v1.ConditionStatus{
		Type:               condType,
		ZoneId:             zoneId,
		Status:             status,
		LastTransitionTime: time,
	}
}
