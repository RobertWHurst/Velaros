package velaros

import "sync"

// InboundMessage represents a message received from a WebSocket client.
// The structure and interpretation of these fields depends on the middleware
// in use (typically JSON middleware which populates these fields from the
// incoming message format).
//
// Middleware can modify these fields via Context methods (SetMessageID,
// SetMessagePath, SetMessageData) to transform messages before they reach handlers.
type InboundMessage struct {
	// hasSetID tracks whether the ID was modified by middleware, used internally
	// for routing decisions.
	hasSetID bool

	// hasSetPath tracks whether the Path was modified by middleware, used internally
	// for routing decisions.
	hasSetPath bool

	// ID is the message identifier, used for request/reply correlation. If the
	// incoming message doesn't include an ID, one is automatically generated.
	ID string

	// Path is the message path used for routing to handlers. Patterns are matched
	// against this path to determine which handlers should execute.
	Path string

	// RawData contains the raw message payload as bytes before any middleware processing.
	// This is the unmodified data as received from the WebSocket connection. Middleware
	// can parse this and call ctx.SetMessageData() to populate the Data field.
	RawData []byte

	// Data contains the processed message payload as bytes after middleware extraction.
	// Middleware may extract this from a nested 'data' field in the message envelope.
	// Handlers can unmarshal this using ctx.ReceiveInto() if middleware has configured
	// an unmarshaler.
	Data []byte

	// Meta contains optional metadata attached to the message. Applications can use
	// this to pass authentication tokens, tracing IDs, or other contextual information.
	Meta map[string]any
}

var inboundMessagePool = sync.Pool{
	New: func() any {
		return &InboundMessage{}
	},
}

func inboundMessageFromPool() *InboundMessage {
	msg := inboundMessagePool.Get().(*InboundMessage)
	msg.hasSetID = false
	msg.hasSetPath = false
	msg.ID = ""
	msg.Path = ""
	msg.RawData = nil
	msg.Data = nil
	msg.Meta = nil
	return msg
}

func (m *InboundMessage) free() {
	inboundMessagePool.Put(m)
}

// OutboundMessage represents a message being sent to a WebSocket client.
// The marshalling of this structure into bytes is handled by middleware
// (typically JSON middleware).
type OutboundMessage struct {
	// ID is the message identifier. When set, it enables the client to correlate
	// this message with a previous request they sent. Used by Send() to echo back
	// the client's message ID for request/reply correlation, and by Request() to
	// track responses.
	ID string

	// Data is the message payload. This will be marshalled by middleware (typically
	// to JSON) before being sent over the WebSocket connection.
	Data any
}
