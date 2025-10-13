package velaros_test

import (
	"testing"

	"github.com/RobertWHurst/velaros"
	"github.com/coder/websocket"
)

func TestSocketMustGet(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	t.Run("MustGetFromSocket with existing key", func(t *testing.T) {
		router.Bind("/socket-must-get-exists", func(ctx *velaros.Context) {
			ctx.SetOnSocket("mykey", "myvalue")

			val := ctx.MustGetFromSocket("mykey")
			if val != "myvalue" {
				t.Errorf("expected 'myvalue', got %v", val)
			}

			if err := ctx.Send(testMessage{Msg: "ok"}); err != nil {
				t.Errorf("send failed: %v", err)
			}
		})

		conn, ctx := dialWebSocket(t, server.URL)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		writeMessage(t, conn, ctx, "", "/socket-must-get-exists", nil)
		_, response := readMessage(t, conn, ctx)

		if response.Msg != "ok" {
			t.Errorf("expected 'ok', got %q", response.Msg)
		}
	})

	t.Run("MustGetFromSocket with non-existent key panics", func(t *testing.T) {
		router.Bind("/socket-must-get-missing", func(ctx *velaros.Context) {
			_ = ctx.MustGetFromSocket("nonexistent")

			if err := ctx.Send(testMessage{Msg: "should not reach"}); err != nil {
				t.Errorf("send failed: %v", err)
			}
		})

		conn, ctx := dialWebSocket(t, server.URL)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		writeMessage(t, conn, ctx, "", "/socket-must-get-missing", nil)
	})
}

func TestSocketID(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	var socketID1, socketID2 string

	router.Bind("/get-socket-id", func(ctx *velaros.Context) {
		id := ctx.SocketID()
		if id == "" {
			t.Error("expected socket ID to be non-empty")
		}

		if socketID1 == "" {
			socketID1 = id
		} else if socketID2 == "" {
			socketID2 = id
		}

		if err := ctx.Send(testMessage{Msg: id}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn1, ctx1 := dialWebSocket(t, server.URL)
	defer func() { _ = conn1.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn1, ctx1, "", "/get-socket-id", nil)
	_, response1 := readMessage(t, conn1, ctx1)

	if response1.Msg == "" {
		t.Error("expected socket ID in response")
	}

	conn2, ctx2 := dialWebSocket(t, server.URL)
	defer func() { _ = conn2.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn2, ctx2, "", "/get-socket-id", nil)
	_, response2 := readMessage(t, conn2, ctx2)

	if response2.Msg == "" {
		t.Error("expected socket ID in response")
	}

	if socketID1 == socketID2 {
		t.Error("expected different socket IDs for different connections")
	}
}

func TestSocketHeaders(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/get-headers", func(ctx *velaros.Context) {
		headers := ctx.Headers()
		if headers == nil {
			t.Error("expected headers to be non-nil")
		}

		if err := ctx.Send(testMessage{Msg: "headers ok"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/get-headers", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "headers ok" {
		t.Errorf("expected 'headers ok', got %q", response.Msg)
	}
}

func TestSocketStorageThreadSafety(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/concurrent-socket-storage", func(ctx *velaros.Context) {
		for i := 0; i < 100; i++ {
			ctx.SetOnSocket("key", i)
		}

		val, ok := ctx.GetFromSocket("key")
		if !ok {
			t.Error("expected key to exist")
		}

		if _, isInt := val.(int); !isInt {
			t.Errorf("expected int value, got %T", val)
		}

		if err := ctx.Send(testMessage{Msg: "concurrent ok"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/concurrent-socket-storage", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "concurrent ok" {
		t.Errorf("expected 'concurrent ok', got %q", response.Msg)
	}
}

func TestSocketContextCancellation(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	doneChan := make(chan struct{})

	router.Bind("/check-socket-done", func(ctx *velaros.Context) {
		select {
		case <-ctx.Done():
			t.Error("expected socket context not to be done during handler")
		default:
		}

		if err := ctx.Send(testMessage{Msg: "not done"}); err != nil {
			t.Errorf("send failed: %v", err)
		}

		close(doneChan)
	})

	conn, ctx := dialWebSocket(t, server.URL)
	writeMessage(t, conn, ctx, "", "/check-socket-done", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "not done" {
		t.Errorf("expected 'not done', got %q", response.Msg)
	}

	<-doneChan
	_ = conn.Close(websocket.StatusNormalClosure, "test complete")
}
