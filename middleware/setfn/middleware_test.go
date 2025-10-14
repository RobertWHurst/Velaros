package setfn_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/RobertWHurst/velaros/middleware/setfn"
	"github.com/coder/websocket"
)

type testMessage struct {
	Value string `json:"value"`
}

func TestMiddleware(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	var counter atomic.Int64
	router.Use(setfn.Middleware("requestID", func() int64 {
		return counter.Add(1)
	}))

	router.Bind("/test", func(ctx *velaros.Context) {
		id, ok := ctx.Get("requestID")
		if !ok {
			t.Error("expected requestID to be set")
		}
		if id.(int64) < 1 {
			t.Errorf("expected requestID >= 1, got %d", id)
		}

		if err := ctx.Send(testMessage{Value: "ok"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	msg := map[string]string{"path": "/test"}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
		t.Fatal(err)
	}

	_, respBytes, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Data testMessage `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		t.Fatal(err)
	}

	if response.Data.Value != "ok" {
		t.Errorf("expected 'ok', got %q", response.Data.Value)
	}
}

func TestMiddlewareFunctionCalledPerMessage(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	var callCount atomic.Int64
	router.Use(setfn.Middleware("requestID", func() int64 {
		return callCount.Add(1)
	}))

	router.Bind("/test", func(ctx *velaros.Context) {
		id := ctx.MustGet("requestID").(int64)
		if err := ctx.Send(testMessage{Value: string(rune('0' + id))}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	for i := 0; i < 3; i++ {
		msg := map[string]string{"path": "/test"}
		msgBytes, _ := json.Marshal(msg)
		if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
			t.Fatal(err)
		}
		_, _, _ = conn.Read(ctx)
	}

	if callCount.Load() != 3 {
		t.Errorf("expected function to be called 3 times, got %d", callCount.Load())
	}
}
