package velaros_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
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
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
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
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("marshal reply failed: %v", err)
	}
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
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("marshal reply failed: %v", err)
	}
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
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("marshal reply failed: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, replyBytes); err != nil {
		t.Fatalf("write failed: %v", err)
	}

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
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("marshal reply failed: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, replyBytes); err != nil {
		t.Fatalf("write failed: %v", err)
	}

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
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("marshal reply failed: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, replyBytes); err != nil {
		t.Fatalf("write failed: %v", err)
	}

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
	replyBytes, err := json.Marshal(reply)
	if err != nil {
		t.Fatalf("marshal reply failed: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, replyBytes); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, response := readMessage(t, conn, ctx)
	if response.Msg != "Cancelled" {
		t.Errorf("expected 'Cancelled', got %q", response.Msg)
	}
}

func TestRouterUseOpen(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	openCalled := false
	var capturedSocketID string

	router.UseOpen(func(ctx *velaros.Context) {
		openCalled = true
		capturedSocketID = ctx.SocketID()
		ctx.SetOnSocket("connectedAt", "test-value")
	})

	router.Bind("/test", func(ctx *velaros.Context) {
		value, ok := ctx.GetFromSocket("connectedAt")
		if !ok {
			t.Error("expected connectedAt to be set on socket")
		}
		if value != "test-value" {
			t.Errorf("expected connectedAt to be 'test-value', got %v", value)
		}
		if err := ctx.Send(testMessage{Msg: "ok"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/test", nil)
	readMessage(t, conn, ctx)

	if !openCalled {
		t.Error("expected UseOpen handler to be called")
	}

	if capturedSocketID == "" {
		t.Error("expected socket ID to be captured")
	}
}

func TestRouterUseClose(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())
	server := httptest.NewServer(router)
	defer server.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	closeCalled := false
	var capturedSocketID string

	router.UseOpen(func(ctx *velaros.Context) {
		ctx.SetOnSocket("sessionData", "cleanup-me")
	})

	router.UseClose(func(ctx *velaros.Context) {
		closeCalled = true
		capturedSocketID = ctx.SocketID()

		value, ok := ctx.GetFromSocket("sessionData")
		if !ok {
			t.Error("expected sessionData to be available in close handler")
		}
		if value != "cleanup-me" {
			t.Errorf("expected sessionData to be 'cleanup-me', got %v", value)
		}
		wg.Done()
	})

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close(websocket.StatusNormalClosure, "")

	wg.Wait()

	if !closeCalled {
		t.Error("expected UseClose handler to be called")
	}

	if capturedSocketID == "" {
		t.Error("expected socket ID to be captured")
	}
}

func TestRouterUseOpenAndClose(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())
	server := httptest.NewServer(router)
	defer server.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	openCalled := false
	closeCalled := false
	var openSocketID, closeSocketID string

	router.UseOpen(func(ctx *velaros.Context) {
		openCalled = true
		openSocketID = ctx.SocketID()
	})

	router.UseClose(func(ctx *velaros.Context) {
		closeCalled = true
		closeSocketID = ctx.SocketID()
		wg.Done()
	})

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close(websocket.StatusNormalClosure, "")

	wg.Wait()

	if !openCalled {
		t.Error("expected UseOpen handler to be called")
	}

	if !closeCalled {
		t.Error("expected UseClose handler to be called")
	}

	if openSocketID == "" {
		t.Error("expected open socket ID to be captured")
	}

	if closeSocketID == "" {
		t.Error("expected close socket ID to be captured")
	}

	if openSocketID != closeSocketID {
		t.Errorf("expected same socket ID in open and close, got %s and %s", openSocketID, closeSocketID)
	}
}

func TestRouterMultipleUseOpen(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())
	server := httptest.NewServer(router)
	defer server.Close()

	firstCalled := false
	secondCalled := false

	router.UseOpen(func(ctx *velaros.Context) {
		firstCalled = true
		ctx.SetOnSocket("first", "done")
		ctx.Next()
	})

	router.UseOpen(func(ctx *velaros.Context) {
		secondCalled = true
		if _, ok := ctx.GetFromSocket("first"); !ok {
			t.Error("expected first handler to have run")
		}
		ctx.SetOnSocket("second", "done")
	})

	router.Bind("/test", func(ctx *velaros.Context) {
		if _, ok := ctx.GetFromSocket("first"); !ok {
			t.Error("expected first handler to have set value")
		}
		if _, ok := ctx.GetFromSocket("second"); !ok {
			t.Error("expected second handler to have set value")
		}
		if err := ctx.Send(map[string]string{"status": "ok"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	msg := []byte(`{"path": "/test"}`)
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatal(err)
	}

	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !firstCalled {
		t.Error("expected first UseOpen handler to be called")
	}

	if !secondCalled {
		t.Error("expected second UseOpen handler to be called")
	}
}

func TestContextClose(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	closeChan := make(chan struct{})

	router.Bind("/close-me", func(ctx *velaros.Context) {
		ctx.Close()
	})

	router.UseClose(func(ctx *velaros.Context) {
		close(closeChan)
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/close-me", nil)

	select {
	case <-closeChan:
	case <-time.After(10 * time.Second):
		t.Error("expected UseClose handler to be called after ctx.Close()")
	}
}

func TestContextCloseStopsMessageLoop(t *testing.T) {
	router, server := setupRouter()
	defer server.Close()

	messagesReceived := 0

	router.Bind("/first", func(ctx *velaros.Context) {
		messagesReceived++
		if err := ctx.Send(testMessage{Msg: "first"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
		ctx.Close()
	})

	router.Bind("/second", func(ctx *velaros.Context) {
		messagesReceived++
		if err := ctx.Send(testMessage{Msg: "second"}); err != nil {
			t.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(t, server.URL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	writeMessage(t, conn, ctx, "", "/first", nil)
	readMessage(t, conn, ctx)

	writeMessage(t, conn, ctx, "", "/second", nil)

	time.Sleep(50 * time.Millisecond)

	if messagesReceived != 1 {
		t.Errorf("expected only 1 message to be received, got %d", messagesReceived)
	}
}
