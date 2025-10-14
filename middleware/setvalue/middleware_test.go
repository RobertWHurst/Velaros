package setvalue_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/RobertWHurst/velaros/middleware/setvalue"
	"github.com/coder/websocket"
)

type testMessage struct {
	Value int `json:"value"`
}

type Config struct {
	MaxRetries int
	Timeout    int
}

func TestMiddleware(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	config := &Config{MaxRetries: 3, Timeout: 30}
	router.Use(setvalue.Middleware("config", config))

	router.Bind("/test", func(ctx *velaros.Context) {
		cfg, ok := ctx.Get("config")
		if !ok {
			t.Error("expected config to be set")
		}

		configVal := cfg.(Config)
		if configVal.MaxRetries != 3 {
			t.Errorf("expected MaxRetries 3, got %d", configVal.MaxRetries)
		}
		if configVal.Timeout != 30 {
			t.Errorf("expected Timeout 30, got %d", configVal.Timeout)
		}

		if err := ctx.Send(testMessage{Value: configVal.MaxRetries}); err != nil {
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

	if response.Data.Value != 3 {
		t.Errorf("expected 3, got %d", response.Data.Value)
	}
}

func TestMiddlewareStoresValueNotPointer(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	timeout := 30
	router.Use(setvalue.Middleware("timeout", &timeout))

	router.Bind("/test", func(ctx *velaros.Context) {
		val := ctx.MustGet("timeout").(int)
		if val != 60 {
			t.Errorf("expected current dereferenced value 60, got %d", val)
		}
		if err := ctx.Send(testMessage{Value: val}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	timeout = 60

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
	_, _, _ = conn.Read(ctx)
}
