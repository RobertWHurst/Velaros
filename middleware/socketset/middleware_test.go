package socketset_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/RobertWHurst/velaros/middleware/socketset"
	"github.com/coder/websocket"
)

type testMessage struct {
	Value string `json:"value"`
}

func TestMiddleware(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())
	router.Use(socketset.Middleware("serverVersion", "1.0.0"))
	router.Use(socketset.Middleware("maxConnections", 100))

	router.Bind("/test", func(ctx *velaros.Context) {
		version, ok := ctx.GetFromSocket("serverVersion")
		if !ok {
			t.Error("expected serverVersion to be set")
		}
		if version != "1.0.0" {
			t.Errorf("expected '1.0.0', got %v", version)
		}

		maxConn, ok := ctx.GetFromSocket("maxConnections")
		if !ok {
			t.Error("expected maxConnections to be set")
		}
		if maxConn != 100 {
			t.Errorf("expected 100, got %v", maxConn)
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

func TestMiddlewareSetsValueOnSocket(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.Use(socketset.Middleware("config", "production"))

	router.Bind("/test", func(ctx *velaros.Context) {
		config := ctx.MustGetFromSocket("config").(string)
		if config != "production" {
			t.Errorf("expected 'production', got %q", config)
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

	for i := 0; i < 2; i++ {
		msg := map[string]string{"path": "/test"}
		msgBytes, _ := json.Marshal(msg)
		if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
			t.Fatal(err)
		}
		_, _, _ = conn.Read(ctx)
	}
}
