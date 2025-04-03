package local

type LocalState int

const (
	NewLocal LocalState = iota
	Starting
	Running
	Terminated
	UnknownLocal
)

func (s LocalState) String() string {
	switch s {
	case NewLocal:
		return "New"
	case Starting:
		return "Starting"
	case Running:
		return "Running"
	case Terminated:
		return "Terminated"
	default:
		return "Unknown"
	}
}
