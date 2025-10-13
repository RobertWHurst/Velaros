package velaros_test

import (
	"context"
	"testing"
	"time"

	"github.com/RobertWHurst/velaros"
	"github.com/coder/websocket"
)

func TestContextGetSet(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/get-set", func(ctx *velaros.Context) {
		// Test Set and Get
		ctx.Set("key1", "value1")
		ctx.Set("key2", 42)
		ctx.Set("key3", struct{ Name string }{Name: "test"})

		val1, ok1 := ctx.Get("key1")
		if !ok1 || val1 != "value1" {
			t.Errorf("expected key1='value1', got ok=%v, val=%v", ok1, val1)
		}

		val2, ok2 := ctx.Get("key2")
		if !ok2 || val2 != 42 {
			t.Errorf("expected key2=42, got ok=%v, val=%v", ok2, val2)
		}

		_, ok3 := ctx.Get("key3")
		if !ok3 {
			t.Error("expected key3 to exist")
		}

		// Test non-existent key
		_, ok := ctx.Get("nonexistent")
		if ok {
			t.Error("expected non-existent key to return false")
		}

		if err := ctx.Send(testMessage{Msg: "success"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/get-set", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "success" {
		t.Errorf("expected 'success', got %q", response.Msg)
	}
}

func TestContextMustGet(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	t.Run("MustGet with existing key", func(t *testing.T) {
		router.Bind("/must-get-exists", func(ctx *velaros.Context) {
			ctx.Set("mykey", "myvalue")

			val := ctx.MustGet("mykey")
			if val != "myvalue" {
				t.Errorf("expected 'myvalue', got %v", val)
			}

			if err := ctx.Send(testMessage{Msg: "ok"}); err != nil {
				t.Errorf("send failed: %v", err)
			}
		})

		conn, ctx := dialWebSocket(t, server.URL)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		writeMessage(t, conn, ctx, "", "/must-get-exists", nil)
		_, response := readMessage(t, conn, ctx)

		if response.Msg != "ok" {
			t.Errorf("expected 'ok', got %q", response.Msg)
		}
	})

	t.Run("MustGet with non-existent key panics", func(t *testing.T) {
		panicked := make(chan bool, 1)

		router.Bind("/must-get-missing", func(ctx *velaros.Context) {
			defer func() {
				if r := recover(); r != nil {
					panicked <- true
				}
			}()

			_ = ctx.MustGet("nonexistent")

			if err := ctx.Send(testMessage{Msg: "should not reach here"}); err != nil {
				t.Errorf("send failed: %v", err)
			}
		})

		conn, ctx := dialWebSocket(t, server.URL)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		writeMessage(t, conn, ctx, "", "/must-get-missing", nil)

		select {
		case <-panicked:
			// Good - panic was caught
		case <-time.After(100 * time.Millisecond):
			t.Error("expected MustGet to panic for non-existent key")
		}
	})
}

func TestContextGetFromSocketSetOnSocket(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/socket-storage", func(ctx *velaros.Context) {
		// Set value on socket
		ctx.SetOnSocket("connection-id", "conn-123")
		ctx.SetOnSocket("user", "alice")

		// Get value from socket
		val1, ok1 := ctx.GetFromSocket("connection-id")
		if !ok1 || val1 != "conn-123" {
			t.Errorf("expected connection-id='conn-123', got ok=%v, val=%v", ok1, val1)
		}

		val2, ok2 := ctx.GetFromSocket("user")
		if !ok2 || val2 != "alice" {
			t.Errorf("expected user='alice', got ok=%v, val=%v", ok2, val2)
		}

		// Test non-existent key
		_, ok := ctx.GetFromSocket("nonexistent")
		if ok {
			t.Error("expected non-existent socket key to return false")
		}

		if err := ctx.Send(testMessage{Msg: "socket storage works"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/socket-storage", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "socket storage works" {
		t.Errorf("expected 'socket storage works', got %q", response.Msg)
	}
}

func TestContextSocketStoragePersistence(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/set-socket-val", func(ctx *velaros.Context) {
		ctx.SetOnSocket("persistent", "data")
		if err := ctx.Send(testMessage{Msg: "set"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	router.Bind("/get-socket-val", func(ctx *velaros.Context) {
		val, ok := ctx.GetFromSocket("persistent")
		if !ok {
			t.Error("expected persistent value to exist")
		}
		if val != "data" {
			t.Errorf("expected 'data', got %v", val)
		}
		if err := ctx.Send(testMessage{Msg: "got"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// First message: set value
	writeMessage(t, conn, ctx, "", "/set-socket-val", nil)
	_, response1 := readMessage(t, conn, ctx)
	if response1.Msg != "set" {
		t.Errorf("expected 'set', got %q", response1.Msg)
	}

	// Second message: get value (should persist across messages)
	writeMessage(t, conn, ctx, "", "/get-socket-val", nil)
	_, response2 := readMessage(t, conn, ctx)
	if response2.Msg != "got" {
		t.Errorf("expected 'got', got %q", response2.Msg)
	}
}

func TestContextParams(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/users/:userId/posts/:postId", func(ctx *velaros.Context) {
		params := ctx.Params()

		if params == nil {
			t.Error("expected params to be non-nil")
		}

		userId := params.Get("userId")
		if userId != "42" {
			t.Errorf("expected userId='42', got %q", userId)
		}

		postId := params.Get("postId")
		if postId != "101" {
			t.Errorf("expected postId='101', got %q", postId)
		}

		if err := ctx.Send(testMessage{Msg: "params work"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/users/42/posts/101", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "params work" {
		t.Errorf("expected 'params work', got %q", response.Msg)
	}
}

func TestContextDeadline(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/deadline-test", func(ctx *velaros.Context) {
		// Default context has no deadline
		_, ok := ctx.Deadline()
		if ok {
			t.Error("expected no deadline on default context")
		}

		if err := ctx.Send(testMessage{Msg: "no deadline"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/deadline-test", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "no deadline" {
		t.Errorf("expected 'no deadline', got %q", response.Msg)
	}
}

func TestContextErr(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/err-test", func(ctx *velaros.Context) {
		// Context should not be canceled during handler execution
		if ctx.Err() != nil {
			t.Errorf("expected no error, got %v", ctx.Err())
		}

		if err := ctx.Send(testMessage{Msg: "no error"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/err-test", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "no error" {
		t.Errorf("expected 'no error', got %q", response.Msg)
	}
}

func TestContextValue(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/value-test", func(ctx *velaros.Context) {
		// Context.Value should return nil for any key
		// (it's a stub implementation for go context.Context interface)
		val := ctx.Value("somekey")
		if val != nil {
			t.Errorf("expected nil, got %v", val)
		}

		if err := ctx.Send(testMessage{Msg: "value is nil"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/value-test", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "value is nil" {
		t.Errorf("expected 'value is nil', got %q", response.Msg)
	}
}

func TestContextDone(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/done-test", func(ctx *velaros.Context) {
		// Done channel should not be closed during handler execution
		select {
		case <-ctx.Done():
			t.Error("expected Done channel to be open during handler execution")
		default:
			// Good - not done yet
		}

		if err := ctx.Send(testMessage{Msg: "not done"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/done-test", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "not done" {
		t.Errorf("expected 'not done', got %q", response.Msg)
	}
}

func TestContextCanBeUsedWithGoContextFunctions(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/go-context-test", func(ctx *velaros.Context) {
		// This tests that our Context properly implements context.Context
		// and can be used with standard library context functions
		derivedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Derived context should have a deadline
		_, ok := derivedCtx.Deadline()
		if !ok {
			t.Error("expected derived context to have a deadline")
		}

		// Derived context should not be done yet
		select {
		case <-derivedCtx.Done():
			t.Error("expected derived context not to be done yet")
		default:
			// Good
		}

		if err := ctx.Send(testMessage{Msg: "go context works"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/go-context-test", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "go context works" {
		t.Errorf("expected 'go context works', got %q", response.Msg)
	}
}

func TestContextMessageLevelStorageDoesNotPersist(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/set-message-val", func(ctx *velaros.Context) {
		ctx.Set("message-data", "temporary")
		if err := ctx.Send(testMessage{Msg: "set"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	router.Bind("/get-message-val", func(ctx *velaros.Context) {
		_, ok := ctx.Get("message-data")
		if ok {
			t.Error("expected message-level data NOT to persist across messages")
		}
		if err := ctx.Send(testMessage{Msg: "not found"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// First message: set value
	writeMessage(t, conn, ctx, "", "/set-message-val", nil)
	_, response1 := readMessage(t, conn, ctx)
	if response1.Msg != "set" {
		t.Errorf("expected 'set', got %q", response1.Msg)
	}

	// Second message: try to get value (should NOT persist)
	writeMessage(t, conn, ctx, "", "/get-message-val", nil)
	_, response2 := readMessage(t, conn, ctx)
	if response2.Msg != "not found" {
		t.Errorf("expected 'not found', got %q", response2.Msg)
	}
}

func TestContextSetMessageData(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Use(func(ctx *velaros.Context) {
		// Modify message data in middleware
		ctx.SetMessageData([]byte(`{"modified": true}`))
		ctx.Next()
	})

	router.Bind("/modify-data", func(ctx *velaros.Context) {
		data := ctx.Data()
		expected := []byte(`{"modified": true}`)
		if string(data) != string(expected) {
			t.Errorf("expected data to be modified, got %s", string(data))
		}

		if err := ctx.Send(testMessage{Msg: "data modified"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/modify-data", testMessage{Msg: "original"})
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "data modified" {
		t.Errorf("expected 'data modified', got %q", response.Msg)
	}
}
