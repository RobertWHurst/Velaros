package protobuf

import (
	"encoding/json"
	"errors"

	"github.com/RobertWHurst/velaros"
	"google.golang.org/protobuf/proto"
)

// Middleware provides Protocol Buffers message format support for Velaros WebSocket connections.
// Define standard .proto schemas, generate Go code with protoc, and use your protobuf types directly
// with Velaros - no special wrapper types needed. The middleware handles envelope wrapping/unwrapping
// transparently.
//
// # Message Format
//
// Internally, messages are wrapped in an envelope containing:
//   - id: Message identifier for request/reply correlation
//   - path: Routing path (gRPC-style "/package.Service/Method" or custom "/api/users/get")
//   - data: Your serialized protobuf message bytes
//
// The envelope is handled automatically - you only work with your .proto message types.
//
// # Usage
//
//	// Define your standard .proto schema (no Velaros-specific types needed)
//	// syntax = "proto3";
//	// message GetUserRequest { int64 user_id = 1; }
//	// message GetUserResponse { string name = 1; }
//
//	// Generate with: protoc --go_out=. user.proto
//
//	router := velaros.NewRouter()
//	router.Use(protobuf.Middleware())
//
//	router.Bind("/users/get", func(ctx *velaros.Context) {
//	    var req userpb.GetUserRequest
//	    ctx.Unmarshal(&req)  // Works like standard protobuf
//
//	    user := getUser(req.UserId)
//	    ctx.Reply(&userpb.GetUserResponse{
//	        Name:  user.Name,
//	        Email: user.Email,
//	    })
//	})
//
// # Client Integration
//
// Your clients need to wrap protobuf messages in the Velaros envelope format.
// The envelope.proto schema is available in this package for client code generation.
//
// JavaScript example:
//
//	// Load envelope.proto and your schemas
//	const Envelope = root.lookupType("velaros.protobuf.Envelope");
//	const GetUserRequest = root.lookupType("users.GetUserRequest");
//
//	// Create and encode your message
//	const request = GetUserRequest.create({ userId: 123 });
//	const requestBytes = GetUserRequest.encode(request).finish();
//
//	// Wrap in envelope
//	const envelope = Envelope.create({
//	    id: "msg-123",
//	    path: "/users/get",
//	    data: requestBytes
//	});
//
//	// Send envelope
//	ws.send(Envelope.encode(envelope).finish());
//
// The middleware validates the Sec-WebSocket-Protocol header, expecting "velaros-protobuf" if present.
func Middleware() func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		headers := ctx.Headers()
		secWebSocketProtocol := headers.Get("Sec-WebSocket-Protocol")
		if secWebSocketProtocol != "" && secWebSocketProtocol != "velaros-protobuf" {
			ctx.Error = errors.New("Unsupported WebSocket Subprotocol: " + secWebSocketProtocol)
			return
		}

		var messageData Envelope
		if err := proto.Unmarshal(ctx.Data(), &messageData); err != nil {
			if secWebSocketProtocol == "" {
				ctx.Next()
				return
			}
			ctx.Error = err
			return
		}

		if messageData.Id != "" {
			ctx.SetMessageID(messageData.Id)
		}
		if messageData.Path != "" {
			ctx.SetMessagePath(messageData.Path)
		}

		if len(messageData.Meta) > 0 {
			meta := make(map[string]any)
			for key, value := range messageData.Meta {
				var deserializedValue any
				if err := json.Unmarshal(value, &deserializedValue); err != nil {
					meta[key] = value
				} else {
					meta[key] = deserializedValue
				}
			}
			ctx.SetMessageMeta(meta)
		}

		ctx.SetMessageUnmarshaler(func(message *velaros.InboundMessage, into any) error {
			protoMsg, ok := into.(proto.Message)
			if !ok {
				return errors.New("value must implement proto.Message (generated protobuf struct)")
			}
			return proto.Unmarshal(messageData.Data, protoMsg)
		})

		ctx.SetMessageMarshaller(func(message *velaros.OutboundMessage) ([]byte, error) {
			protoMsg, ok := message.Data.(proto.Message)
			if !ok {
				return nil, errors.New("value must implement proto.Message (generated protobuf struct)")
			}

			data, err := proto.Marshal(protoMsg)
			if err != nil {
				return nil, err
			}

			envelope := &Envelope{
				Id:   message.ID,
				Data: data,
			}


			return proto.Marshal(envelope)
		})

		ctx.Next()
	}
}
