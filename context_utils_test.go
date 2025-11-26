package velaros_test

import (
	"encoding/json"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
)

func Test_CtxSocket(t *testing.T) {
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	inboundMsg := &velaros.InboundMessage{RawData: []byte("{}")}

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	retrievedSocket := velaros.CtxSocket(ctx)
	if retrievedSocket != socket {
		t.Error("expected CtxSocket to return the same socket instance")
	}
	if retrievedSocket.ID() != socket.ID() {
		t.Errorf("expected socket ID %s, got %s", socket.ID(), retrievedSocket.ID())
	}
}

func Test_CtxMeta_WithMeta(t *testing.T) {
	msgData := map[string]any{
		"id":   "test-msg",
		"path": "/test",
		"meta": map[string]any{
			"userId":  "user-123",
			"traceId": "trace-456",
		},
	}
	msgBytes, _ := json.Marshal(msgData)

	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	inboundMsg := &velaros.InboundMessage{RawData: msgBytes}

	var capturedMeta map[string]any
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		capturedMeta = velaros.CtxMeta(ctx)
	})

	middleware := jsonMiddleware.Middleware()
	middleware(ctx)

	if capturedMeta == nil {
		t.Fatal("expected CtxMeta to return meta map")
	}

	if capturedMeta["userId"] != "user-123" {
		t.Errorf("expected userId=user-123, got %v", capturedMeta["userId"])
	}
	if capturedMeta["traceId"] != "trace-456" {
		t.Errorf("expected traceId=trace-456, got %v", capturedMeta["traceId"])
	}
}

func Test_CtxMeta_WithoutMeta(t *testing.T) {
	msgData := map[string]any{
		"id":   "test-msg",
		"path": "/test",
	}
	msgBytes, _ := json.Marshal(msgData)

	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	inboundMsg := &velaros.InboundMessage{RawData: msgBytes}

	var capturedMeta map[string]any
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		capturedMeta = velaros.CtxMeta(ctx)
	})

	middleware := jsonMiddleware.Middleware()
	middleware(ctx)

	if capturedMeta != nil {
		t.Errorf("expected CtxMeta to return nil, got %v", capturedMeta)
	}
}

func Test_CtxFree(t *testing.T) {
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	inboundMsg := &velaros.InboundMessage{RawData: []byte("{}")}

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	velaros.CtxFree(ctx)

	if ctx.Error != nil {
		t.Errorf("unexpected error after free: %v", ctx.Error)
	}
}
