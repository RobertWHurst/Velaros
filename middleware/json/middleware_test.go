package json

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/RobertWHurst/velaros"
)

func TestJSONMiddleware_ValidMessage(t *testing.T) {
	msgData := map[string]any{
		"id":      "msg-123",
		"path":    "/users/get",
		"user_id": 42,
		"name":    "Alice",
	}
	msgBytes, _ := json.Marshal(msgData)

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
		UserID int    `json:"user_id"`
		Name   string `json:"name"`
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

func TestJSONMiddleware_MissingID(t *testing.T) {
	msgData := map[string]any{
		"path": "/test",
		"data": map[string]string{"msg": "hello"},
	}
	msgBytes, _ := json.Marshal(msgData)

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

func TestJSONMiddleware_MissingPath(t *testing.T) {
	msgData := map[string]any{
		"id":   "msg-456",
		"data": map[string]string{"msg": "hello"},
	}
	msgBytes, _ := json.Marshal(msgData)

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

func TestJSONMiddleware_InvalidJSON(t *testing.T) {
	inboundMsg := &velaros.InboundMessage{Data: []byte("invalid json {{")}
	socket := velaros.NewSocket(http.Header{}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if nextCalled {
		t.Error("expected Next() not to be called on error")
	}
}

func TestJSONMiddleware_ProtocolValidation_Valid(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "velaros-json")
	socket := velaros.NewSocket(headers, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}

func TestJSONMiddleware_ProtocolValidation_Invalid(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := json.Marshal(msgData)

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

func TestJSONMiddleware_ProtocolValidation_Empty(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(http.Header{}, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}
