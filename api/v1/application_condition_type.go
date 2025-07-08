package v1

import (
	"encoding/json"
	"errors"
)

type ApplicationConditionType string

const (
	LocalConditionType             ApplicationConditionType = "Local"
	PlacementConditionType         ApplicationConditionType = "Placement"
	OwnershipTransferConditionType ApplicationConditionType = "OwnershipTransfer"
	DeploymenConditionType         ApplicationConditionType = "Deployment"
	UndeploymenConditionType       ApplicationConditionType = "Undeployment"
)

func (s *ApplicationConditionType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(LocalConditionType),
		string(PlacementConditionType),
		string(OwnershipTransferConditionType),
		string(DeploymenConditionType),
		string(UndeploymenConditionType):
		*s = ApplicationConditionType(str)
		return nil
	default:
		return errors.New("invalid ApplicationConditionType: " + str)
	}
}

func (s ApplicationConditionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type OwnershipTransferStatus string

const (
	OwnershipTransferPulling OwnershipTransferStatus = "Pulling"
	OwnershipTransferFailure OwnershipTransferStatus = "Failure"
	OwnershipTransferSuccess OwnershipTransferStatus = "Success"
)

func (s *OwnershipTransferStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(OwnershipTransferPulling),
		string(OwnershipTransferFailure),
		string(OwnershipTransferSuccess):
		*s = OwnershipTransferStatus(str)
		return nil
	default:
		return errors.New("invalid OwnershipTransferStatus: " + str)
	}
}

func (s OwnershipTransferStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type PlacementStatus string

const (
	PlacementStatusInProgress PlacementStatus = "InProgress"
	PlacementStatusDone       PlacementStatus = "Done"
	PlacementStatusFailure    PlacementStatus = "Failure"
)

func (s *PlacementStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(PlacementStatusDone),
		string(PlacementStatusFailure):
		*s = PlacementStatus(str)
		return nil
	default:
		return errors.New("invalid PlacementStatus: " + str)
	}
}

func (s PlacementStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type DeploymentStatus string

const (
	DeploymentStatusPull    DeploymentStatus = "Pull"
	DeploymentStatusDone    DeploymentStatus = "Done"
	DeploymentStatusFailure DeploymentStatus = "Failure"
)

func (s *DeploymentStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(DeploymentStatusPull),
		string(DeploymentStatusDone),
		string(DeploymentStatusFailure):
		*s = DeploymentStatus(str)
		return nil
	default:
		return errors.New("invalid DeploymentStatus: " + str)
	}
}

func (s DeploymentStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type UndeploymentStatus string

const (
	UndeploymentStatusUndeploy UndeploymentStatus = "Undeploy"
	UndeploymentStatusDone     UndeploymentStatus = "Done"
	UndeploymentStatusFailure  UndeploymentStatus = "Failure"
)

func (s *UndeploymentStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(UndeploymentStatusUndeploy),
		string(UndeploymentStatusDone),
		string(UndeploymentStatusFailure):
		*s = UndeploymentStatus(str)
		return nil
	default:
		return errors.New("invalid UndeploymentStatus: " + str)
	}
}

func (s UndeploymentStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
