package v1

import (
	"encoding/json"
	"errors"
)

type GlobalState string

const (
	UnknownGlobalState           GlobalState = "Unknown"
	NewGlobalState               GlobalState = "New"
	PlacementGlobalState         GlobalState = "Placement"
	OperationalGlobalState       GlobalState = "Operational"
	RelocationGlobalState        GlobalState = "Relocation"
	FailureGlobalState           GlobalState = "Failure"
	OwnershipTransferGlobalState GlobalState = "OwnershipTransfer"
)

func (s *GlobalState) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(UnknownGlobalState),
		string(NewGlobalState),
		string(PlacementGlobalState),
		string(OperationalGlobalState),
		string(RelocationGlobalState),
		string(FailureGlobalState),
		string(OwnershipTransferGlobalState):
		*s = GlobalState(str)
		return nil
	default:
		return errors.New("invalid status value " + str)
	}
}

func (s GlobalState) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
