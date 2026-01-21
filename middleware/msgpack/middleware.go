package msgpack

import (
	"errors"

	"github.com/RobertWHurst/velaros"
	"github.com/vmihailenco/msgpack/v5"
)

// Middleware provides MessagePack message format support for Velaros WebSocket connections.
// It handles parsing incoming MessagePack messages and serializing outgoing messages according
// to the Velaros MessagePack message envelope format.
//
// # Message Format
//
// Incoming messages should be MessagePack objects with the following structure:
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
//	router.Use(msgpack.Middleware())
//
//	router.Bind("/users/:id", func(ctx *velaros.Context) {
//	    var req GetUserRequest
//	    ctx.ReceiveInto(&req)  // Automatically uses MessagePack unmarshaling
//	    ctx.Send(user)       // Automatically serialized to MessagePack
//	})
//
// The middleware also validates the Sec-WebSocket-Protocol header, expecting
// "velaros-msgpack" if present.
func Middleware() func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		headers := ctx.Headers()
		secWebSocketProtocol := headers.Get("Sec-WebSocket-Protocol")
		if secWebSocketProtocol != "" && secWebSocketProtocol != "velaros-msgpack" {
			ctx.Error = errors.New("Unsupported WebSocket Subprotocol: " + secWebSocketProtocol)
			return
		}

		var messageData struct {
			ID   string             `msgpack:"id"`
			Path string             `msgpack:"path"`
			Meta map[string]any     `msgpack:"meta"`
			Data msgpack.RawMessage `msgpack:"data"`
		}
		if err := msgpack.Unmarshal(ctx.RawData(), &messageData); err != nil {
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
			return msgpack.Unmarshal(message.Data, into)
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

			return msgpack.Marshal(envelope)
		})

		ctx.Next()
	}
}
