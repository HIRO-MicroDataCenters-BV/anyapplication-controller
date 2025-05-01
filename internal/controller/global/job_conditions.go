package global

import (
	v1 "hiro.io/anyapplication/api/v1"
)

type JobId struct {
	Type   v1.ApplicationConditionType
	ZoneId string
}

type JobApplicationConditions struct {
	Conditions []*v1.ConditionStatus
}

func EmptyJobConditions() JobApplicationConditions {
	return JobApplicationConditions{}
}

func FromCondition(condition *v1.ConditionStatus) JobApplicationConditions {
	return JobApplicationConditions{
		Conditions: []*v1.ConditionStatus{
			condition,
		},
	}
}
