package v1

import (
	"encoding/json"
	"errors"
)

type ApplicationConditionType string

const (
	LocalConditionType     ApplicationConditionType = "Local"
	PlacementConditionType ApplicationConditionType = "Placement"
	OwnershipTransfer      ApplicationConditionType = "OwnershipTransfer"
	Relocation             ApplicationConditionType = "Relocation"
)

func (s *ApplicationConditionType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(LocalConditionType),
		string(PlacementConditionType),
		string(OwnershipTransfer),
		string(Relocation):
		*s = ApplicationConditionType(str)
		return nil
	default:
		return errors.New("invalid status value")
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
		return errors.New("invalid status value")
	}
}

func (s OwnershipTransferStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type PlacementStatus string

const (
	PlacementStatusDone    OwnershipTransferStatus = "Done"
	PlacementStatusFailure OwnershipTransferStatus = "Failure"
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
		return errors.New("invalid status value")
	}
}

func (s PlacementStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
