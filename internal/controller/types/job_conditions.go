package types

import (
	v1 "hiro.io/anyapplication/api/v1"
)

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
