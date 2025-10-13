package protobuf

import (
	"net/http"
	"testing"

	"github.com/RobertWHurst/velaros"
	"google.golang.org/protobuf/proto"
)

func TestProtobufMiddleware_ValidMessage(t *testing.T) {
	testReq := &TestRequest{
		UserId: 42,
		Name:   "Alice",
	}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Id:   "msg-123",
		Path: "/users/get",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{Data: envBytes}
	socket := velaros.NewSocket(http.Header{}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	if ctx.MessageID() != "msg-123" {
		t.Errorf("expected MessageID 'msg-123', got '%s'", ctx.MessageID())
	}
	if ctx.Path() != "/users/get" {
		t.Errorf("expected Path '/users/get', got '%s'", ctx.Path())
	}

	if !nextCalled {
		t.Error("expected Next() to be called")
	}

	var result TestRequest
	if err := ctx.Unmarshal(&result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.UserId != 42 {
		t.Errorf("expected UserId 42, got %d", result.UserId)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Name 'Alice', got %s", result.Name)
	}
}

func TestProtobufMiddleware_MissingID(t *testing.T) {
	testReq := &TestRequest{UserId: 1, Name: "Bob"}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Path: "/test",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{Data: envBytes}
	socket := velaros.NewSocket(http.Header{}, nil)
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	if ctx.Path() != "/test" {
		t.Errorf("expected Path '/test', got '%s'", ctx.Path())
	}
}

func TestProtobufMiddleware_MissingPath(t *testing.T) {
	testReq := &TestRequest{UserId: 1, Name: "Charlie"}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Id:   "msg-456",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{Data: envBytes}
	socket := velaros.NewSocket(http.Header{}, nil)
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	if ctx.MessageID() != "msg-456" {
		t.Errorf("expected MessageID 'msg-456', got '%s'", ctx.MessageID())
	}

	if ctx.Path() != "" {
		t.Errorf("expected empty Path, got '%s'", ctx.Path())
	}
}

func TestProtobufMiddleware_InvalidProtobuf(t *testing.T) {
	inboundMsg := &velaros.InboundMessage{Data: []byte("invalid protobuf \xFF\xFE")}
	socket := velaros.NewSocket(http.Header{}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error == nil {
		t.Fatal("expected error for invalid protobuf")
	}

	if nextCalled {
		t.Error("expected Next() not to be called on error")
	}
}

func TestProtobufMiddleware_NonProtoMessage(t *testing.T) {
	testReq := &TestRequest{UserId: 1, Name: "Dave"}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Path: "/test",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{Data: envBytes}
	socket := velaros.NewSocket(http.Header{}, nil)
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	var result map[string]any
	if err := ctx.Unmarshal(&result); err == nil {
		t.Error("expected error when unmarshaling into non-proto.Message")
	}
}

func TestProtobufMiddleware_ProtocolValidation_Valid(t *testing.T) {
	testReq := &TestRequest{UserId: 1}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{Path: "/test", Data: reqBytes}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{Data: envBytes}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "velaros-protobuf")
	socket := velaros.NewSocket(headers, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}

func TestProtobufMiddleware_ProtocolValidation_Invalid(t *testing.T) {
	testReq := &TestRequest{UserId: 1}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{Path: "/test", Data: reqBytes}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{Data: envBytes}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "wrong-protocol")
	socket := velaros.NewSocket(headers, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error == nil {
		t.Fatal("expected error for invalid protocol")
	}

	if nextCalled {
		t.Error("expected Next() not to be called on protocol error")
	}
}

func TestProtobufMiddleware_ProtocolValidation_Empty(t *testing.T) {
	testReq := &TestRequest{UserId: 1}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{Path: "/test", Data: reqBytes}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{Data: envBytes}
	socket := velaros.NewSocket(http.Header{}, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}
