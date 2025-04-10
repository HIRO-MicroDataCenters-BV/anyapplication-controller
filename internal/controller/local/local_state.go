package local

type LocalState int

const (
	UnknownLocal LocalState = iota
	NewLocal
	Starting
	Running
	Completed
)

func (s LocalState) String() string {
	switch s {
	case NewLocal:
		return "New"
	case Starting:
		return "Starting"
	case Running:
		return "Running"
	case Completed:
		return "Terminated"
	default:
		return "Unknown"
	}
}

type ApplicationConditionType string

const (
	LocalStatusType ApplicationConditionType = "LocalStatus"
)
