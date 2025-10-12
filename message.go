package velaros

type InboundMessage struct {
	hasSetID   bool
	hasSetPath bool
	ID         string
	Path       string
	Data       []byte
}

type OutboundMessage struct {
	ID   string
	Data any
}
