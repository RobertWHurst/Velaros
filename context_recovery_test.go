package velaros_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/RobertWHurst/velaros"
	"github.com/coder/websocket"
)

func TestPanicRecoveryInHandler(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	panicMessage := "intentional panic for testing"

	router.Bind("/panic", func(ctx *velaros.Context) {
		panic(panicMessage)
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Send message to panic handler
	msg := []byte(`{"path": "/panic"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatal(err)
	}

	// Connection should still be alive after panic
	// Try to send another message to verify
	msg2 := []byte(`{"path": "/panic"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg2); err != nil {
		t.Error("connection died after panic - expected it to stay alive")
	}
}

func TestPanicRecoveryInMiddleware(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	var mu sync.Mutex
	panicInMiddleware := false
	handlerCalled := false

	router.Use(func(ctx *velaros.Context) {
		if ctx.Path() == "/panic-middleware" {
			mu.Lock()
			panicInMiddleware = true
			mu.Unlock()
			panic("middleware panic")
		}
		ctx.Next()
	})

	router.Bind("/panic-middleware", func(ctx *velaros.Context) {
		mu.Lock()
		handlerCalled = true
		mu.Unlock()
		if err := ctx.Send(testMessage{Msg: "success"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	msg := []byte(`{"path": "/panic-middleware"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatal(err)
	}

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !panicInMiddleware {
		t.Error("expected middleware to panic")
	}

	if handlerCalled {
		t.Error("expected handler not to be called after middleware panic")
	}
}

func TestPanicRecoveryMultiplePanics(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	var mu sync.Mutex
	panicCount := 0

	router.Bind("/panic-multiple", func(ctx *velaros.Context) {
		mu.Lock()
		panicCount++
		currentCount := panicCount
		mu.Unlock()
		panic("panic #" + string(rune('0'+currentCount)))
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Send multiple messages that will panic - server should handle all gracefully
	for i := 0; i < 3; i++ {
		msg := []byte(`{"path": "/panic-multiple"}`)
		if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if panicCount != 3 {
		t.Errorf("expected 3 panics, got %d", panicCount)
	}
}

func TestPanicRecoveryDoesNotAffectOtherHandlers(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/panic", func(ctx *velaros.Context) {
		panic("boom")
	})

	router.Bind("/good", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "all good"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// First, trigger panic
	msg1 := []byte(`{"path": "/panic"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg1); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Now call good handler - should work fine
	msg2 := []byte(`{"path": "/good"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg2); err != nil {
		t.Fatal(err)
	}

	// Read response
	_, msgBytes, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Data testMessage `json:"data"`
	}
	if err := json.Unmarshal(msgBytes, &response); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if response.Data.Msg != "all good" {
		t.Errorf("expected 'all good', got %q", response.Data.Msg)
	}
}

func TestPanicInNextChain(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	var mu sync.Mutex
	middleware1Called := false
	middleware2Called := false
	handlerCalled := false

	router.Use(func(ctx *velaros.Context) {
		mu.Lock()
		middleware1Called = true
		mu.Unlock()
		ctx.Next()
	})

	router.Use(func(ctx *velaros.Context) {
		mu.Lock()
		middleware2Called = true
		mu.Unlock()
		panic("panic in middleware 2")
	})

	router.Bind("/chain-panic", func(ctx *velaros.Context) {
		mu.Lock()
		handlerCalled = true
		mu.Unlock()
		if err := ctx.Send(testMessage{Msg: "success"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	msg := []byte(`{"path": "/chain-panic"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !middleware1Called {
		t.Error("expected middleware1 to be called")
	}

	if !middleware2Called {
		t.Error("expected middleware2 to be called")
	}

	if handlerCalled {
		t.Error("expected handler not to be called after middleware panic")
	}
}

func TestPanicWithDifferentTypes(t *testing.T) {
	tests := []struct {
		name        string
		panicValue  any
		expectError bool
	}{
		{
			name:        "panic with string",
			panicValue:  "string panic",
			expectError: true,
		},
		{
			name:        "panic with error",
			panicValue:  errors.New("error panic"),
			expectError: true,
		},
		{
			name:        "panic with int",
			panicValue:  42,
			expectError: true,
		},
		{
			name:        "panic with nil",
			panicValue:  nil,
			expectError: true,
		},
		{
			name:        "panic with struct",
			panicValue:  struct{ msg string }{msg: "struct panic"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, server := setupRouter()
			defer server.Close()

			router.Bind("/panic-type", func(ctx *velaros.Context) {
				panic(tt.panicValue)
			})

			conn, ctx := dialWebSocket(t, server.URL)
			defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

			msg := []byte(`{"path": "/panic-type"}`)
			if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
				t.Fatal(err)
			}

			// Connection should still be alive
			time.Sleep(50 * time.Millisecond)

			// Verify we can still send messages
			msg2 := []byte(`{"path": "/panic-type"}`)
			if err := conn.Write(ctx, websocket.MessageText, msg2); err != nil {
				t.Error("connection died after panic")
			}
		})
	}
}

func TestNoPanicInNormalOperation(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	handlerCalled := false

	router.Bind("/normal", func(ctx *velaros.Context) {
		handlerCalled = true
		if err := ctx.Send(testMessage{Msg: "normal operation"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	msg := []byte(`{"path": "/normal"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatal(err)
	}

	// Read response
	_, msgBytes, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Data testMessage `json:"data"`
	}
	if err := json.Unmarshal(msgBytes, &response); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called")
	}

	if response.Data.Msg != "normal operation" {
		t.Errorf("expected 'normal operation', got %q", response.Data.Msg)
	}
}
