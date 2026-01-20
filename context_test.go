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
		case <-time.After(100 * time.Millisecond):
			t.Error("expected MustGet to panic for non-existent key")
		}
	})
}

func TestContextGetFromSocketSetOnSocket(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/socket-storage", func(ctx *velaros.Context) {
		ctx.SetOnSocket("connection-id", "conn-123")
		ctx.SetOnSocket("user", "alice")

		val1, ok1 := ctx.GetFromSocket("connection-id")
		if !ok1 || val1 != "conn-123" {
			t.Errorf("expected connection-id='conn-123', got ok=%v, val=%v", ok1, val1)
		}

		val2, ok2 := ctx.GetFromSocket("user")
		if !ok2 || val2 != "alice" {
			t.Errorf("expected user='alice', got ok=%v, val=%v", ok2, val2)
		}

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

	writeMessage(t, conn, ctx, "", "/set-socket-val", nil)
	_, response1 := readMessage(t, conn, ctx)
	if response1.Msg != "set" {
		t.Errorf("expected 'set', got %q", response1.Msg)
	}

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
		select {
		case <-ctx.Done():
			t.Error("expected Done channel to be open during handler execution")
		default:
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
		derivedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		_, ok := derivedCtx.Deadline()
		if !ok {
			t.Error("expected derived context to have a deadline")
		}

		select {
		case <-derivedCtx.Done():
			t.Error("expected derived context not to be done yet")
		default:
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

func TestContextReceive(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/conversation", func(ctx *velaros.Context) {
		// Send initial greeting
		if err := ctx.Send(testMessage{Msg: "Hello! What's your name?"}); err != nil {
			t.Errorf("reply failed: %v", err)
			return
		}

		// Receive first message
		var msg1 testMessage
		if err := ctx.ReceiveInto(&msg1); err != nil {
			t.Errorf("receive failed: %v", err)
			return
		}

		// Send response
		if err := ctx.Send(testMessage{Msg: "Nice to meet you, " + msg1.Msg}); err != nil {
			t.Errorf("reply failed: %v", err)
			return
		}

		// Receive second message
		var msg2 testMessage
		if err := ctx.ReceiveInto(&msg2); err != nil {
			t.Errorf("receive failed: %v", err)
			return
		}

		// Send final response
		if err := ctx.Send(testMessage{Msg: "You said: " + msg2.Msg}); err != nil {
			t.Errorf("reply failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Start conversation with an ID
	conversationID := "conv-123"
	writeMessage(t, conn, ctx, conversationID, "/conversation", nil)

	// Read greeting
	id1, response1 := readMessage(t, conn, ctx)
	if id1 != conversationID {
		t.Errorf("expected ID %q, got %q", conversationID, id1)
	}
	if response1.Msg != "Hello! What's your name?" {
		t.Errorf("unexpected greeting: %q", response1.Msg)
	}

	// Send first message with same ID
	writeMessage(t, conn, ctx, conversationID, "/conversation", testMessage{Msg: "Alice"})

	// Read response
	id2, response2 := readMessage(t, conn, ctx)
	if id2 != conversationID {
		t.Errorf("expected ID %q, got %q", conversationID, id2)
	}
	if response2.Msg != "Nice to meet you, Alice" {
		t.Errorf("unexpected response: %q", response2.Msg)
	}

	// Send second message with same ID
	writeMessage(t, conn, ctx, conversationID, "/conversation", testMessage{Msg: "Goodbye"})

	// Read final response
	id3, response3 := readMessage(t, conn, ctx)
	if id3 != conversationID {
		t.Errorf("expected ID %q, got %q", conversationID, id3)
	}
	if response3.Msg != "You said: Goodbye" {
		t.Errorf("unexpected response: %q", response3.Msg)
	}
}

func TestContextReceiveRaw(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/raw-receive", func(ctx *velaros.Context) {
		// Send initial message
		if err := ctx.Send(testMessage{Msg: "ready"}); err != nil {
			t.Errorf("send failed: %v", err)
			return
		}

		// Receive raw data
		data, err := ctx.Receive()
		if err != nil {
			t.Errorf("receive failed: %v", err)
			return
		}

		// Data should be the raw JSON bytes
		if len(data) == 0 {
			t.Error("expected non-empty data")
		}

		// Send back the raw data
		if err := ctx.Send(testMessage{Msg: string(data)}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	msgID := "raw-123"
	writeMessage(t, conn, ctx, msgID, "/raw-receive", nil)

	// Read ready message
	_, response1 := readMessage(t, conn, ctx)
	if response1.Msg != "ready" {
		t.Errorf("expected 'ready', got %q", response1.Msg)
	}

	// Send message with raw data
	writeMessage(t, conn, ctx, msgID, "/raw-receive", testMessage{Msg: "test data"})

	// Read response with raw data echoed back
	_, response2 := readMessage(t, conn, ctx)
	if len(response2.Msg) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestContextReceiveTimeout(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/timeout", func(ctx *velaros.Context) {
		var msg testMessage
		err := ctx.ReceiveIntoWithTimeout(&msg, 100*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error, got nil")
		}

		if err := ctx.Send(testMessage{Msg: "timed out"}); err != nil {
			t.Errorf("reply failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "test-id", "/timeout", nil)

	// Don't send follow-up message, let it timeout

	_, response := readMessage(t, conn, ctx)
	if response.Msg != "timed out" {
		t.Errorf("expected 'timed out', got %q", response.Msg)
	}
}

func TestContextReceiveCleanup(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	handlerComplete := make(chan bool, 1)

	router.Bind("/cleanup", func(ctx *velaros.Context) {
		// Interceptor is auto-created, but we don't call Receive
		if err := ctx.Send(testMessage{Msg: "done"}); err != nil {
			t.Errorf("reply failed: %v", err)
		}
		handlerComplete <- true
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "cleanup-id", "/cleanup", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "done" {
		t.Errorf("expected 'done', got %q", response.Msg)
	}

	// Wait for handler to complete
	select {
	case <-handlerComplete:
		// Success - interceptor was cleaned up when context freed
	case <-time.After(1 * time.Second):
		t.Error("handler did not complete")
	}

	// Send another message with same ID - should start a NEW handler
	writeMessage(t, conn, ctx, "cleanup-id", "/cleanup", nil)
	_, response2 := readMessage(t, conn, ctx)

	if response2.Msg != "done" {
		t.Errorf("expected 'done', got %q", response2.Msg)
	}
}

func TestContextRequest(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/echo", func(ctx *velaros.Context) {
		var msg testMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			t.Errorf("unmarshal failed: %v", err)
			return
		}

		if err := ctx.Send(testMessage{Msg: "echo: " + msg.Msg}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	router.Bind("/requester", func(ctx *velaros.Context) {
		// Send a request and get raw response
		data, err := ctx.Request(testMessage{Msg: "hello"})
		if err != nil {
			t.Errorf("request failed: %v", err)
			return
		}

		if len(data) == 0 {
			t.Error("expected non-empty response data")
		}

		if err := ctx.Send(testMessage{Msg: "got response"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	writeMessage(t, conn, ctx, "req-123", "/requester", nil)

	// First message is the Request to /echo
	id1, msg1 := readMessage(t, conn, ctx)
	if id1 != "req-123" {
		t.Errorf("expected ID %q, got %q", "req-123", id1)
	}
	if msg1.Msg != "hello" {
		t.Errorf("expected 'hello', got %q", msg1.Msg)
	}

	// Send the echo response back
	writeMessage(t, conn, ctx, id1, "/echo", testMessage{Msg: "echo: hello"})

	// Read final response
	_, response := readMessage(t, conn, ctx)
	if response.Msg != "got response" {
		t.Errorf("expected 'got response', got %q", response.Msg)
	}
}

func TestContextRequestInto(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/echo", func(ctx *velaros.Context) {
		var msg testMessage
		if err := ctx.Unmarshal(&msg); err != nil {
			t.Errorf("unmarshal failed: %v", err)
			return
		}

		if err := ctx.Send(testMessage{Msg: "echo: " + msg.Msg}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	router.Bind("/requester", func(ctx *velaros.Context) {
		// Send a request and unmarshal response
		var response testMessage
		if err := ctx.RequestInto(testMessage{Msg: "world"}, &response); err != nil {
			t.Errorf("request failed: %v", err)
			return
		}

		if response.Msg != "echo: world" {
			t.Errorf("expected 'echo: world', got %q", response.Msg)
		}

		if err := ctx.Send(testMessage{Msg: "success"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	writeMessage(t, conn, ctx, "req-456", "/requester", nil)

	// First message is the Request to /echo
	id1, msg1 := readMessage(t, conn, ctx)
	if id1 != "req-456" {
		t.Errorf("expected ID %q, got %q", "req-456", id1)
	}
	if msg1.Msg != "world" {
		t.Errorf("expected 'world', got %q", msg1.Msg)
	}

	// Send the echo response back
	writeMessage(t, conn, ctx, id1, "/echo", testMessage{Msg: "echo: world"})

	// Read final response
	_, response := readMessage(t, conn, ctx)
	if response.Msg != "success" {
		t.Errorf("expected 'success', got %q", response.Msg)
	}
}

func TestContextRequestTimeout(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/requester", func(ctx *velaros.Context) {
		var response testMessage
		err := ctx.RequestIntoWithTimeout(testMessage{Msg: "timeout test"}, &response, 100*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error, got nil")
		}

		if err := ctx.Send(testMessage{Msg: "timed out"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "timeout-789", "/requester", nil)

	// Read the request message
	_, msg := readMessage(t, conn, ctx)
	if msg.Msg != "timeout test" {
		t.Errorf("expected 'timeout test', got %q", msg.Msg)
	}

	// Don't send response, let it timeout

	// Read timeout message
	_, response := readMessage(t, conn, ctx)
	if response.Msg != "timed out" {
		t.Errorf("expected 'timed out', got %q", response.Msg)
	}
}
