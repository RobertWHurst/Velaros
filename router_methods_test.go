package velaros_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/coder/websocket"
)

func TestRouterSetOrigins(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.SetOrigins([]string{"https://example.com", "https://test.com"})

	router.Bind("/test", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "hello"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/test", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "hello" {
		t.Errorf("expected 'hello', got %q", response.Msg)
	}
}

func TestRouterMiddlewareReturnsFunction(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	middlewareFunc := router.Middleware()

	if middlewareFunc == nil {
		t.Error("expected Middleware() to return a non-nil function")
	}
}

func TestRouterPublicBind(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.PublicBind("/api/users", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "users list"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	router.PublicBind("/api/posts/:id", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "post details"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	router.Bind("/internal/admin", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "admin"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	conn, ctx := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeMessage(t, conn, ctx, "", "/api/users", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "users list" {
		t.Errorf("expected 'users list', got %q", response.Msg)
	}

	writeMessage(t, conn, ctx, "", "/api/posts/123", nil)
	_, response2 := readMessage(t, conn, ctx)

	if response2.Msg != "post details" {
		t.Errorf("expected 'post details', got %q", response2.Msg)
	}
}

func TestRouterRouteDescriptors(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.PublicBind("/api/users", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "users"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	router.PublicBind("/api/posts/:id", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "post"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	router.Bind("/internal", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "internal"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	descriptors := router.RouteDescriptors()

	if len(descriptors) != 2 {
		t.Errorf("expected 2 public route descriptors, got %d", len(descriptors))
	}

	foundUsers := false
	foundPosts := false

	for _, desc := range descriptors {
		pattern := desc.Pattern.String()
		if pattern == "/api/users" {
			foundUsers = true
		}
		if pattern == "/api/posts/:id" {
			foundPosts = true
		}
	}

	if !foundUsers {
		t.Error("expected to find /api/users in descriptors")
	}

	if !foundPosts {
		t.Error("expected to find /api/posts/:id in descriptors")
	}
}

func TestRouterServeHTTPNonWebSocket(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.Bind("/ws", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "websocket"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/ws")
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != 400 {
		t.Errorf("expected status 400 for non-WebSocket request, got %d", resp.StatusCode)
	}
}
