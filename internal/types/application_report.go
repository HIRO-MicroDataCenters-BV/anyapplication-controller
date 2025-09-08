// SPDX-FileCopyrightText: 2025 HIRO-MicroDataCenters BV affiliate company and DCP contributors
// SPDX-License-Identifier: Apache-2.0

package types

type PodEvent struct {
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type PodInfo struct {
	Name     string     `json:"name"`
	Status   string     `json:"status"`
	Restarts int32      `json:"restarts"`
	Events   []PodEvent `json:"events"`
	Logs     []LogInfo  `json:"logs"`
}

type LogInfo struct {
	Container string `json:"container"`
	Log       string `json:"log"`
}

type WorkloadStatus struct {
	Kind        string `json:"kind"`      // e.g., Deployment
	Name        string `json:"name"`      // workload name
	Namespace   string `json:"namespace"` // ns
	Ready       bool   `json:"ready"`     // summarized status
	Desired     int32  `json:"desired"`
	Available   int32  `json:"available"`
	Unavailable int32  `json:"unavailable"`
	Message     string `json:"message"` // issue summary
}

type ApplicationReport struct {
	Pods      []PodInfo        `json:"pods"`
	Workloads []WorkloadStatus `json:"workloads"`
}
