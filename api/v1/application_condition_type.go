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
	RelocationConditionType        ApplicationConditionType = "Relocation"
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
		string(RelocationConditionType):
		*s = ApplicationConditionType(str)
		return nil
	default:
		return errors.New("invalid status value: " + str)
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
		return errors.New("invalid status value: " + str)
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
		return errors.New("invalid status value: " + str)
	}
}

func (s PlacementStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type RelocationStatus string

const (
	RelocationStatusPull     RelocationStatus = "Pull"
	RelocationStatusUndeploy RelocationStatus = "Undeploy"
	RelocationStatusDone     RelocationStatus = "Done"
	RelocationStatusFailure  RelocationStatus = "Failure"
)

func (s *RelocationStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(RelocationStatusPull),
		string(RelocationStatusDone),
		string(RelocationStatusFailure):
		*s = RelocationStatus(str)
		return nil
	default:
		return errors.New("invalid status value: " + str)
	}
}

func (s RelocationStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
