package set_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/RobertWHurst/velaros/middleware/set"
	"github.com/coder/websocket"
)

type testMessage struct {
	Value string `json:"value"`
}

func TestMiddleware(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())
	router.Use(set.Middleware("apiVersion", "v1"))
	router.Use(set.Middleware("config", map[string]int{"timeout": 30}))

	router.Bind("/test", func(ctx *velaros.Context) {
		version, ok := ctx.Get("apiVersion")
		if !ok {
			t.Error("expected apiVersion to be set")
		}
		if version != "v1" {
			t.Errorf("expected 'v1', got %v", version)
		}

		config, ok := ctx.Get("config")
		if !ok {
			t.Error("expected config to be set")
		}
		configMap := config.(map[string]int)
		if configMap["timeout"] != 30 {
			t.Errorf("expected timeout 30, got %d", configMap["timeout"])
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

func TestMiddlewareValueDoesNotPersistAcrossMessages(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())
	router.Use(set.Middleware("counter", 0))

	router.Bind("/increment", func(ctx *velaros.Context) {
		counter := ctx.MustGet("counter").(int)
		ctx.Set("counter", counter+1)
		if err := ctx.Send(testMessage{Value: "incremented"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	router.Bind("/check", func(ctx *velaros.Context) {
		counter := ctx.MustGet("counter").(int)
		if counter != 0 {
			t.Errorf("expected counter to be 0 (reset), got %d", counter)
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

	incrementMsg := map[string]string{"path": "/increment"}
	msgBytes, _ := json.Marshal(incrementMsg)
	if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
		t.Fatal(err)
	}
	_, _, _ = conn.Read(ctx)

	checkMsg := map[string]string{"path": "/check"}
	msgBytes, _ = json.Marshal(checkMsg)
	if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
		t.Fatal(err)
	}
	_, _, _ = conn.Read(ctx)
}
