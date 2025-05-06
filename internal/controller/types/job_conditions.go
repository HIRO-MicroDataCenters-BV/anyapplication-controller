package types

import (
	"github.com/samber/mo"
	v1 "hiro.io/anyapplication/api/v1"
)

type JobApplicationCondition struct {
	condition mo.Option[v1.ConditionStatus]
	jobType   AsyncJobType
}

func EmptyJobConditions() JobApplicationCondition {
	return JobApplicationCondition{}
}

func FromCondition(condition v1.ConditionStatus) JobApplicationCondition {
	return JobApplicationCondition{
		condition: mo.Some(condition),
	}
}

func (j JobApplicationCondition) GetConditions() []*v1.ConditionStatus {
	if j.condition.IsAbsent() {
		return nil
	} else {
		return []*v1.ConditionStatus{j.condition.ToPointer()}
	}
}
func (j JobApplicationCondition) GetJobType() mo.Option[AsyncJobType] {
	if j.condition.IsAbsent() {
		return mo.None[AsyncJobType]()
	} else {
		return mo.Some(j.jobType)
	}
}
