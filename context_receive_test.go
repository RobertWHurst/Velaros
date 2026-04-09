package velaros_test

import (
	"encoding/json"
	"fmt"
	"sync"
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

// TestReceive_RapidSameIDNoSimultaneousHandlers verifies that rapid same-ID messages
// never run multiple handler instances simultaneously — the core race this fix addresses.
func TestReceive_RapidSameIDNoSimultaneousHandlers(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	var active int
	var mu sync.Mutex
	peaked := make(chan int, 10)

	router.Bind("/rapid", func(ctx *velaros.Context) {
		mu.Lock()
		active++
		if active > 1 {
			peaked <- active
		}
		mu.Unlock()

		defer func() {
			mu.Lock()
			active--
			mu.Unlock()
		}()

		var msg testMessage
		ctx.ReceiveInto(&msg) //nolint
	})

	conn := newMockConnection()
	go router.HandleConnection(nil, conn)

	send := func(id, path string, data any) {
		msg := map[string]any{"id": id, "path": path}
		if data != nil {
			msg["data"] = data
		}
		raw, _ := json.Marshal(msg)
		conn.sendIncoming(&velaros.SocketMessage{Type: velaros.MessageText, RawData: raw})
	}

	// Send trigger then multiple rapid same-ID same-path continuations
	for i := 0; i < 5; i++ {
		send("race-id", "/rapid", testMessage{Msg: fmt.Sprintf("msg-%d", i)})
	}

	// Give messages time to process
	time.Sleep(200 * time.Millisecond)

	// Verify no simultaneous handler instances
	select {
	case n := <-peaked:
		t.Errorf("multiple simultaneous handler instances: %d at once", n)
	default:
		// expected: never more than one handler at a time
	}

	conn.Close(velaros.StatusNormalClosure, "done") //nolint
}

// TestReceive_SameIDDifferentPathBothHandled verifies that same-ID messages on
// different paths both get handled (serialized, not dropped).
func TestReceive_SameIDDifferentPathBothHandled(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	results := make(chan string, 4)

	router.Bind("/first", func(ctx *velaros.Context) {
		results <- "first"
	})

	router.Bind("/second", func(ctx *velaros.Context) {
		results <- "second"
	})

	conn := newMockConnection()
	go router.HandleConnection(nil, conn)

	send := func(id, path string) {
		raw, _ := json.Marshal(map[string]any{"id": id, "path": path})
		conn.sendIncoming(&velaros.SocketMessage{Type: velaros.MessageText, RawData: raw})
	}

	send("shared-id", "/first")
	send("shared-id", "/second")

	// Both handlers should run (order is not guaranteed due to goroutine scheduling)
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case got := <-results:
			seen[got] = true
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for handler (got so far: %v)", seen)
		}
	}

	if !seen["first"] {
		t.Error("expected /first handler to have run")
	}
	if !seen["second"] {
		t.Error("expected /second handler to have run")
	}

	conn.Close(velaros.StatusNormalClosure, "done") //nolint
}
