package socketsetfn_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/RobertWHurst/velaros/middleware/socketsetfn"
	"github.com/coder/websocket"
)

type testMessage struct {
	ConnID int64 `json:"connID"`
}

func TestMiddleware(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	var counter atomic.Int64
	router.Use(socketsetfn.Middleware("connID", func() int64 {
		return counter.Add(1)
	}))

	router.Bind("/test", func(ctx *velaros.Context) {
		id, ok := ctx.GetFromSocket("connID")
		if !ok {
			t.Error("expected connID to be set")
		}
		if id.(int64) < 1 {
			t.Errorf("expected connID >= 1, got %d", id)
		}

		if err := ctx.Send(testMessage{ConnID: id.(int64)}); err != nil {
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

	if response.Data.ConnID < 1 {
		t.Errorf("expected connID >= 1, got %d", response.Data.ConnID)
	}
}

func TestMiddlewareFunctionCalledPerMessage(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	var callCount atomic.Int64
	router.Use(socketsetfn.Middleware("connID", func() int64 {
		return callCount.Add(1)
	}))

	router.Bind("/test", func(ctx *velaros.Context) {
		id := ctx.MustGetFromSocket("connID").(int64)
		if err := ctx.Send(testMessage{ConnID: id}); err != nil {
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

		if response.Data.ConnID != int64(i+1) {
			t.Errorf("message %d: expected connID %d, got %d", i, i+1, response.Data.ConnID)
		}
	}
}
