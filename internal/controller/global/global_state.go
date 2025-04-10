package global

type GlobalState string

const (
	UnknownGlobal     GlobalState = "Unknown"
	NewGlobal         GlobalState = "New"
	Placement         GlobalState = "Placement"
	Operational       GlobalState = "Operational"
	Relocation        GlobalState = "Relocation"
	Failure           GlobalState = "Failure"
	OwnershipTransfer GlobalState = "OwnershipTransfer"
)
