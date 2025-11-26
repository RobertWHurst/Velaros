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

	// Data contains the raw message payload as bytes. Handlers can parse this data
	// and call ctx.SetMessageData() to populate the Data field, as well as
	// ctx.SetMessageMeta() to populate the Meta field.
	RawData []byte

	// Data contains the raw message payload as bytes. Handlers can unmarshal this
	// data using ctx.Unmarshal() if middleware has configured an unmarshaler.
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
	// this message with a previous request they sent. Used by Reply() to echo back
	// the client's message ID, and by Request() to generate a new ID for server-initiated
	// requests.
	ID string

	// Data is the message payload. This will be marshalled by middleware (typically
	// to JSON) before being sent over the WebSocket connection.
	Data any
}
