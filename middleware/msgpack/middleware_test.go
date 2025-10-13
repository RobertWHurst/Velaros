package msgpack

import (
	"net/http"
	"testing"

	"github.com/RobertWHurst/velaros"
	"github.com/vmihailenco/msgpack/v5"
)

func TestMessagePackMiddleware_ValidMessage(t *testing.T) {
	msgData := map[string]any{
		"id":      "msg-123",
		"path":    "/users/get",
		"user_id": int64(42),
		"name":    "Alice",
	}
	msgBytes, _ := msgpack.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
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

	var result struct {
		UserID int64  `msgpack:"user_id"`
		Name   string `msgpack:"name"`
	}
	if err := ctx.Unmarshal(&result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", result.UserID)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Name 'Alice', got %s", result.Name)
	}
}

func TestMessagePackMiddleware_MissingID(t *testing.T) {
	msgData := map[string]any{
		"path": "/test",
		"msg":  "hello",
	}
	msgBytes, _ := msgpack.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
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

func TestMessagePackMiddleware_MissingPath(t *testing.T) {
	msgData := map[string]any{
		"id":  "msg-456",
		"msg": "hello",
	}
	msgBytes, _ := msgpack.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
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

func TestMessagePackMiddleware_InvalidMessagePack(t *testing.T) {
	inboundMsg := &velaros.InboundMessage{Data: []byte("invalid msgpack \xFF\xFE")}
	socket := velaros.NewSocket(http.Header{}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error == nil {
		t.Fatal("expected error for invalid MessagePack")
	}

	if nextCalled {
		t.Error("expected Next() not to be called on error")
	}
}

func TestMessagePackMiddleware_ProtocolValidation_Valid(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := msgpack.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "velaros-msgpack")
	socket := velaros.NewSocket(headers, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}

func TestMessagePackMiddleware_ProtocolValidation_Invalid(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := msgpack.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}

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

func TestMessagePackMiddleware_ProtocolValidation_Empty(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := msgpack.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(http.Header{}, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}
