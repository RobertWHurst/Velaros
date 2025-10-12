package json

import (
	"encoding/json"
	"errors"

	"github.com/RobertWHurst/velaros"
)

func Middleware() func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		headers := ctx.Headers()
		secWebSocketProtocol := headers.Get("Sec-WebSocket-Protocol")
		if secWebSocketProtocol != "" && secWebSocketProtocol != "velaros-json" {
			ctx.Error = errors.New("Unsupported WebSocket Subprotocol: " + secWebSocketProtocol)
			return
		}

		var messageData struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		}
		if err := json.Unmarshal(ctx.Data(), &messageData); err != nil {
			ctx.Error = err
			return
		}

		if messageData.ID != "" {
			ctx.SetMessageID(messageData.ID)
		}
		if messageData.Path != "" {
			ctx.SetMessagePath(messageData.Path)
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

func genFieldsField(errors []FieldError) []M {
	var fields []M
	for _, err := range errors {
		field := M{}
		field[err.Field] = err.Error
		fields = append(fields, field)
	}
	return fields
}
