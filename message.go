package scramjet

import (
	"bytes"
)

type InboundMessage struct {
	ID   string        `json:"id"`
	Path string        `json:"path"`
	Data *bytes.Buffer `json:"data"`
}

type OutboundMessage struct {
	ID   string `json:"id"`
	Data any    `json:"data"`
}
