// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"encoding/json"
	"errors"
)

type PlacementStrategy string

const (
	PlacementStrategyLocal  PlacementStrategy = "Local"
	PlacementStrategyGlobal PlacementStrategy = "Global"
)

func (s *PlacementStrategy) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	switch str {
	case string(PlacementStrategyLocal),
		string(PlacementStrategyGlobal):
		*s = PlacementStrategy(str)
		return nil
	default:
		return errors.New("invalid placement strategy: " + str)
	}
}

func (s PlacementStrategy) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}
