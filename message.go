package velaros

type InboundMessage struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Data any    `json:"data"`
}

type OutboundMessage struct {
	ID   string `json:"id,omitempty"`
	Data any    `json:"data,omitempty"`
}
