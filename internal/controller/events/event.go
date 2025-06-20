package events

type Event struct {
	Reason string
	Msg    string
}

const (
	LocalStateChangeReason  string = "Local state change"
	GlobalStateChangeReason string = "Global state change"
)
