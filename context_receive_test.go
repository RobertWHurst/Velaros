package velaros_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/coder/websocket"
)

type handlerResult struct {
	err string
}

// TestReceive_SkipsEmptyMessages_MockConn tests that messages with nil or empty
// data are skipped by ReceiveInto, using a mock connection for deterministic timing.
func TestReceive_SkipsEmptyMessages_MockConn(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	results := make(chan handlerResult, 4)

	router.Bind("/skip", func(ctx *velaros.Context) {
		defer func() { results <- handlerResult{} }()

		var msg1 testMessage
		if err := ctx.ReceiveInto(&msg1); err != nil {
			results <- handlerResult{fmt.Sprintf("first receive: %v", err)}
			return
		}

		if err := ctx.Send(testMessage{Msg: "ready"}); err != nil {
			results <- handlerResult{fmt.Sprintf("ready send: %v", err)}
			return
		}

		var msg2 testMessage
		if err := ctx.ReceiveInto(&msg2); err != nil {
			results <- handlerResult{fmt.Sprintf("second receive: %v", err)}
			return
		}
		if msg2.Msg != "real" {
			results <- handlerResult{fmt.Sprintf("expected 'real', got %q", msg2.Msg)}
			return
		}
	})

	conn := newMockConnection()
	go router.HandleConnection(nil, conn)

	sendMsg := func(id, path string, data any) {
		msg := map[string]any{"id": id, "path": path}
		if data != nil {
			msg["data"] = data
		}
		raw, _ := json.Marshal(msg)
		conn.sendIncoming(&velaros.SocketMessage{Type: velaros.MessageText, RawData: raw})
	}

	sendMsg("s-id", "/skip", testMessage{Msg: "trigger"})
	conn.receiveOutgoing(t, time.Second) // wait for "ready"

	sendMsg("s-id", "/skip", nil)
	sendMsg("s-id", "/skip", testMessage{Msg: "real"})

	select {
	case r := <-results:
		if r.err != "" {
			t.Errorf("handler error: %s", r.err)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for handler")
	}

	conn.Close(velaros.StatusNormalClosure, "done") //nolint
}

// TestReceive_EmptyTriggerFallsThrough verifies that when the trigger message
// has no data, ReceiveInto falls through to wait on the interceptor channel.
func TestReceive_EmptyTriggerFallsThrough(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	results := make(chan handlerResult, 2)

	router.Bind("/empty-trigger", func(ctx *velaros.Context) {
		defer func() { results <- handlerResult{} }()

		if err := ctx.Send(testMessage{Msg: "ready"}); err != nil {
			results <- handlerResult{fmt.Sprintf("ready send: %v", err)}
			return
		}

		var msg testMessage
		if err := ctx.ReceiveInto(&msg); err != nil {
			results <- handlerResult{fmt.Sprintf("receive: %v", err)}
			return
		}
		if msg.Msg != "follow-up" {
			results <- handlerResult{fmt.Sprintf("expected 'follow-up', got %q", msg.Msg)}
			return
		}

		if err := ctx.Send(testMessage{Msg: "got: " + msg.Msg}); err != nil {
			results <- handlerResult{fmt.Sprintf("final send: %v", err)}
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	id := "empty-trig-123"
	writeMessage(t, conn, ctx, id, "/empty-trigger", nil)

	_, ready := readMessage(t, conn, ctx)
	if ready.Msg != "ready" {
		t.Fatalf("expected 'ready', got %q", ready.Msg)
	}

	writeMessage(t, conn, ctx, id, "/empty-trigger", testMessage{Msg: "follow-up"})

	_, response := readMessage(t, conn, ctx)
	if response.Msg != "got: follow-up" {
		t.Errorf("expected 'got: follow-up', got %q", response.Msg)
	}

	select {
	case r := <-results:
		if r.err != "" {
			t.Errorf("handler error: %s", r.err)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for handler")
	}
}

// TestReceive_ContextCancelUnblocksReceive verifies that cancelling the context
// (via timeout) unblocks a pending ReceiveInto.
func TestReceive_ContextCancelUnblocksReceive(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	results := make(chan handlerResult, 2)

	router.Bind("/cancel-receive", func(ctx *velaros.Context) {
		defer func() { results <- handlerResult{} }()

		var trigger testMessage
		if err := ctx.ReceiveInto(&trigger); err != nil {
			results <- handlerResult{fmt.Sprintf("trigger receive: %v", err)}
			return
		}

		var msg testMessage
		err := ctx.ReceiveIntoWithTimeout(&msg, 100*time.Millisecond)
		if err == nil {
			results <- handlerResult{"expected timeout error, got nil"}
		}
	})

	conn := newMockConnection()
	go router.HandleConnection(nil, conn)

	raw, _ := json.Marshal(map[string]any{
		"id":   "cancel-id",
		"path": "/cancel-receive",
		"data": map[string]string{"msg": "trigger"},
	})
	conn.sendIncoming(&velaros.SocketMessage{Type: velaros.MessageText, RawData: raw})

	select {
	case r := <-results:
		if r.err != "" {
			t.Errorf("handler error: %s", r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not complete in time")
	}

	conn.Close(velaros.StatusNormalClosure, "done") //nolint
}
