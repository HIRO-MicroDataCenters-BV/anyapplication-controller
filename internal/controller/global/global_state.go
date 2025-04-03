package global

type GlobalState int

const (
	NewGlobal GlobalState = iota
	Placement
	Operational
	Relocation
	Failure
	OwnershipTransfer
	UnknownGlobal
)

func (s GlobalState) String() string {
	switch s {
	case NewGlobal:
		return "New"
	case Placement:
		return "Placement"
	case Operational:
		return "Operational"
	case Relocation:
		return "Relocation"
	case Failure:
		return "Failure"
	case OwnershipTransfer:
		return "OwnershipTransfer"
	default:
		return "Unknown"
	}
}
