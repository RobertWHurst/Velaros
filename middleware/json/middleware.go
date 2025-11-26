package json

import (
	"encoding/json"
	"errors"

	"github.com/RobertWHurst/velaros"
)

// Middleware provides JSON message format support for Velaros WebSocket connections.
// It handles parsing incoming JSON messages and serializing outgoing messages according
// to the Velaros JSON message envelope format.
//
// # Message Format
//
// Incoming messages should be JSON objects with the following structure:
//
//	{
//	  "id": "optional-message-id",      // For request/reply correlation
//	  "path": "/route/path",            // Required: determines which handler to execute
//	  "data": { /* your data here */ }  // Optional: message payload
//	}
//
// Outgoing messages are automatically wrapped in an envelope:
//
//	{
//	  "id": "message-id",               // Present if replying or making a request
//	  "data": { /* your response */ }   // Your response data
//	}
//
// # Special Response Types
//
// The middleware provides special handling for certain response types:
//
//   - Error (string) -> {"error": "message"}
//   - FieldError/[]FieldError -> {"error": "Validation error", "fields": [...]}
//   - string -> {"message": "your string"}
//   - Other types -> {"data": yourData}
//
// # Usage
//
//	router := velaros.NewRouter()
//	router.Use(json.Middleware())
//
//	router.Bind("/users/:id", func(ctx *velaros.Context) {
//	    var req GetUserRequest
//	    ctx.Unmarshal(&req)  // Automatically uses JSON unmarshaling
//	    ctx.Reply(user)      // Automatically serialized to JSON
//	})
//
// The middleware also validates the Sec-WebSocket-Protocol header, expecting
// "velaros-json" if present.
func Middleware() func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		headers := ctx.Headers()
		secWebSocketProtocol := headers.Get("Sec-WebSocket-Protocol")
		if secWebSocketProtocol != "" && secWebSocketProtocol != "velaros-json" {
			ctx.Error = errors.New("Unsupported WebSocket Subprotocol: " + secWebSocketProtocol)
			return
		}

		var messageData struct {
			ID   string          `json:"id"`
			Path string          `json:"path"`
			Meta map[string]any  `json:"meta"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(ctx.RawData(), &messageData); err != nil {
			if secWebSocketProtocol == "" {
				ctx.Next()
				return
			}
			ctx.Error = err
			return
		}

		if messageData.ID != "" {
			ctx.SetMessageID(messageData.ID)
		}
		if messageData.Path != "" {
			ctx.SetMessagePath(messageData.Path)
		}
		if messageData.Meta != nil {
			ctx.SetMessageMeta(messageData.Meta)
		}
		if messageData.Data != nil {
			ctx.SetMessageData(messageData.Data)
		}

		ctx.SetMessageUnmarshaler(func(message *velaros.InboundMessage, into any) error {
			return json.Unmarshal(message.Data, into)
		})

		ctx.SetMessageMarshaller(func(message *velaros.OutboundMessage) ([]byte, error) {
			switch v := message.Data.(type) {

			case []FieldError:
				message.Data = M{
					"error":  "Validation error",
					"fields": genFieldsField(v),
				}

			case FieldError:
				message.Data = M{
					"error":  "Validation error",
					"fields": genFieldsField([]FieldError{v}),
				}

			case Error:
				message.Data = M{"error": string(v)}

			case string:
				message.Data = M{"message": v}
			}

			envelope := map[string]any{}
			if message.ID != "" {
				envelope["id"] = message.ID
			}
			if message.Data != nil {
				envelope["data"] = message.Data
			}

			return json.Marshal(envelope)
		})

		ctx.Next()
	}
}
