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

// func (j *JobApplicationConditions) GetJobCondition(zoneId string, conditionType v1.ApplicationConditionType) mo.Option[*v1.ConditionStatus] {
// 	jobId := JobId{
// 		Type:   conditionType,
// 		ZoneId: zoneId,
// 	}
// 	return mo.EmptyableToOption(j.JobConditions[jobId])
// }

// func (j *JobApplicationConditions) Iterate() {

// }

func EmptyJobConditions() JobApplicationConditions {
	return JobApplicationConditions{}
}
