// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package events

type Event struct {
	Reason string
	Msg    string
}

const (
	LocalStateChangeReason  string = "Local state change"
	GlobalStateChangeReason string = "Global state change"
)
