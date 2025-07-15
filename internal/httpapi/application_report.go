package httpapi

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

type PodReport struct {
	Pods []PodInfo `json:"pods"`
}
