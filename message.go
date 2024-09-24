package scramjet

import "encoding/json"

type InboundMessage struct {
	ID   string
	Path string
	Data json.RawMessage
}

type OutboundMessage struct {
	ID   string
	Data any
}
