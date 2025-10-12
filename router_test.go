package velaros_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/coder/websocket"
)

type testMessage struct {
	Msg string `json:"msg"`
}

func setupRouter() (*velaros.Router, *httptest.Server) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())
	server := httptest.NewServer(router)
	return router, server
}

func dialWebSocket(t *testing.T, serverURL string) (*websocket.Conn, context.Context) {
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, serverURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn, ctx
}

func writeMessage(t *testing.T, conn *websocket.Conn, ctx context.Context, id, path string, data any) {
	msg := map[string]any{"path": path}
	if id != "" {
		msg["id"] = id
	}
	if data != nil {
		msg["msg"] = data.(testMessage).Msg
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
		t.Fatal(err)
	}
}

func readMessage(t *testing.T, conn *websocket.Conn, ctx context.Context) (id string, data testMessage) {
	_, msgBytes, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		ID   string      `json:"id"`
		Data testMessage `json:"data"`
	}
	if err := json.Unmarshal(msgBytes, &env); err != nil {
		t.Fatalf("unmarshal failed: %v, got: %s", err, string(msgBytes))
	}
	return env.ID, env.Data
}

func TestRouterSimpleHandler(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/echo", func(ctx *velaros.Context) {
		var req struct {
			Msg string `json:"msg"`
		}
		if err := ctx.Unmarshal(&req); err != nil {
			t.Errorf("unmarshal failed: %v", err)
			return
		}
		if err := ctx.Send(testMessage{Msg: "Echo: " + req.Msg}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/echo", testMessage{Msg: "Hello"})
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "Echo: Hello" {
		t.Errorf("expected 'Echo: Hello', got %q", response.Msg)
	}
}

func TestRouterMiddleware(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Use(func(ctx *velaros.Context) {
		ctx.Set("greeting", "Hello World")
		ctx.Next()
	})

	router.Bind("/greet", func(ctx *velaros.Context) {
		greeting := ctx.MustGet("greeting").(string)
		if err := ctx.Send(testMessage{Msg: greeting}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/greet", nil)
	_, response := readMessage(t, conn, ctx)

	if response.Msg != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", response.Msg)
	}
}

func TestRouterReply(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/echo", func(ctx *velaros.Context) {
		var req struct {
			Msg string `json:"msg"`
		}
		if err := ctx.Unmarshal(&req); err != nil {
			t.Errorf("unmarshal failed: %v", err)
			return
		}
		if err := ctx.Reply(testMessage{Msg: "Echo: " + req.Msg}); err != nil {
			t.Errorf("reply failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "request-123", "/echo", testMessage{Msg: "Hello"})
	id, response := readMessage(t, conn, ctx)

	if id != "request-123" {
		t.Errorf("expected ID 'request-123', got %q", id)
	}
	if response.Msg != "Echo: Hello" {
		t.Errorf("expected 'Echo: Hello', got %q", response.Msg)
	}
}

func TestRouterRequest(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	resultChan := make(chan testMessage, 1)
	router.Bind("/ping", func(ctx *velaros.Context) {
		response, err := ctx.Request(testMessage{Msg: "Ping"})
		if err != nil {
			t.Errorf("request failed: %v", err)
			return
		}
		responseData := response.([]byte)
		var msg struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(responseData, &msg); err != nil {
			t.Errorf("unmarshal response failed: %v", err)
			return
		}
		result := testMessage{Msg: "Got: " + msg.Msg}
		resultChan <- result
		if err := ctx.Send(result); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/ping", nil)

	reqID, reqData := readMessage(t, conn, ctx)
	if reqData.Msg != "Ping" {
		t.Fatalf("expected server to send 'Ping', got %q", reqData.Msg)
	}

	reply := map[string]any{
		"id":   reqID,
		"data": testMessage{Msg: "Pong"},
	}
	replyBytes, _ := json.Marshal(reply)
	if err := conn.Write(ctx, websocket.MessageText, replyBytes); err != nil {
		t.Fatal(err)
	}

	_, response := readMessage(t, conn, ctx)
	expected := <-resultChan

	if response.Msg != expected.Msg {
		t.Errorf("expected %q, got %q", expected.Msg, response.Msg)
	}
}

func TestRouterRequestInto(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	resultChan := make(chan testMessage, 1)
	router.Bind("/ping", func(ctx *velaros.Context) {
		var response struct {
			Msg string `json:"msg"`
		}
		if err := ctx.RequestInto(testMessage{Msg: "Ping"}, &response); err != nil {
			t.Errorf("request failed: %v", err)
			return
		}
		result := testMessage{Msg: "Got: " + response.Msg}
		resultChan <- result
		if err := ctx.Send(result); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/ping", nil)

	reqID, reqData := readMessage(t, conn, ctx)
	if reqData.Msg != "Ping" {
		t.Fatalf("expected server to send 'Ping', got %q", reqData.Msg)
	}

	reply := map[string]any{
		"id":   reqID,
		"data": testMessage{Msg: "Pong"},
	}
	replyBytes, _ := json.Marshal(reply)
	if err := conn.Write(ctx, websocket.MessageText, replyBytes); err != nil {
		t.Fatal(err)
	}

	_, response := readMessage(t, conn, ctx)
	expected := <-resultChan

	if response.Msg != expected.Msg {
		t.Errorf("expected %q, got %q", expected.Msg, response.Msg)
	}
}

func TestRouterRequestWithTimeout(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/timeout-test", func(ctx *velaros.Context) {
		_, err := ctx.RequestWithTimeout(testMessage{Msg: "Ping"}, 10*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error, got nil")
			return
		}
		if err := ctx.Send(testMessage{Msg: "Timed out"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/timeout-test", nil)

	reqID, reqData := readMessage(t, conn, ctx)
	if reqData.Msg != "Ping" {
		t.Fatalf("expected server to send 'Ping', got %q", reqData.Msg)
	}

	time.Sleep(50 * time.Millisecond)

	reply := map[string]any{
		"id":   reqID,
		"data": testMessage{Msg: "Too late"},
	}
	replyBytes, _ := json.Marshal(reply)
	conn.Write(ctx, websocket.MessageText, replyBytes)

	_, response := readMessage(t, conn, ctx)
	if response.Msg != "Timed out" {
		t.Errorf("expected 'Timed out', got %q", response.Msg)
	}
}

func TestRouterRequestIntoWithTimeout(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/timeout-test", func(ctx *velaros.Context) {
		var response testMessage
		err := ctx.RequestIntoWithTimeout(testMessage{Msg: "Ping"}, &response, 10*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error, got nil")
			return
		}
		if err := ctx.Send(testMessage{Msg: "Timed out"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/timeout-test", nil)

	reqID, reqData := readMessage(t, conn, ctx)
	if reqData.Msg != "Ping" {
		t.Fatalf("expected server to send 'Ping', got %q", reqData.Msg)
	}

	time.Sleep(50 * time.Millisecond)

	reply := map[string]any{
		"id":   reqID,
		"data": testMessage{Msg: "Too late"},
	}
	replyBytes, _ := json.Marshal(reply)
	conn.Write(ctx, websocket.MessageText, replyBytes)

	_, response := readMessage(t, conn, ctx)
	if response.Msg != "Timed out" {
		t.Errorf("expected 'Timed out', got %q", response.Msg)
	}
}

func TestRouterRequestWithContext(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/cancel-test", func(ctx *velaros.Context) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, err := ctx.RequestWithContext(cancelCtx, testMessage{Msg: "Ping"})
		if err == nil {
			t.Error("expected context cancelled error, got nil")
			return
		}
		if err := ctx.Send(testMessage{Msg: "Cancelled"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/cancel-test", nil)

	reqID, reqData := readMessage(t, conn, ctx)
	if reqData.Msg != "Ping" {
		t.Fatalf("expected server to send 'Ping', got %q", reqData.Msg)
	}

	reply := map[string]any{
		"id":   reqID,
		"data": testMessage{Msg: "Too late"},
	}
	replyBytes, _ := json.Marshal(reply)
	conn.Write(ctx, websocket.MessageText, replyBytes)

	_, response := readMessage(t, conn, ctx)
	if response.Msg != "Cancelled" {
		t.Errorf("expected 'Cancelled', got %q", response.Msg)
	}
}

func TestRouterRequestIntoWithContext(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/cancel-test", func(ctx *velaros.Context) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		var response testMessage
		err := ctx.RequestIntoWithContext(cancelCtx, testMessage{Msg: "Ping"}, &response)
		if err == nil {
			t.Error("expected context cancelled error, got nil")
			return
		}
		if err := ctx.Send(testMessage{Msg: "Cancelled"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/cancel-test", nil)

	reqID, reqData := readMessage(t, conn, ctx)
	if reqData.Msg != "Ping" {
		t.Fatalf("expected server to send 'Ping', got %q", reqData.Msg)
	}

	reply := map[string]any{
		"id":   reqID,
		"data": testMessage{Msg: "Too late"},
	}
	replyBytes, _ := json.Marshal(reply)
	conn.Write(ctx, websocket.MessageText, replyBytes)

	_, response := readMessage(t, conn, ctx)
	if response.Msg != "Cancelled" {
		t.Errorf("expected 'Cancelled', got %q", response.Msg)
	}
}
