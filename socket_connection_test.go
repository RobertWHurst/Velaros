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

type mockConnection struct {
	incomingMessages chan *velaros.SocketMessage
	outgoingMessages chan *velaros.SocketMessage
	closed           bool
	mu               sync.Mutex
}

func newMockConnection() *mockConnection {
	return &mockConnection{
		incomingMessages: make(chan *velaros.SocketMessage, 10),
		outgoingMessages: make(chan *velaros.SocketMessage, 10),
	}
}

func (m *mockConnection) Read(ctx context.Context) (*velaros.SocketMessage, error) {
	select {
	case msg := <-m.incomingMessages:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockConnection) Write(ctx context.Context, msg *velaros.SocketMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return context.Canceled
	}
	select {
	case m.outgoingMessages <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *mockConnection) Close(status velaros.Status, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	close(m.incomingMessages)
	return nil
}

func (m *mockConnection) sendIncoming(msg *velaros.SocketMessage) {
	m.incomingMessages <- msg
}

func (m *mockConnection) receiveOutgoing(t *testing.T, timeout time.Duration) *velaros.SocketMessage {
	select {
	case msg := <-m.outgoingMessages:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for outgoing message")
		return nil
	}
}

func TestCustomConnection_MessageMetaFlowThrough(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.Bind("/test", func(ctx *velaros.Context) {
		userId, ok := ctx.Meta("userId")
		if !ok {
			t.Error("expected userId to be present in meta")
			return
		}
		if userIdStr, ok := userId.(string); !ok || userIdStr != "user123" {
			t.Errorf("expected userId=user123 in meta, got: %v", userId)
		}

		if err := ctx.Send(map[string]string{"status": "ok"}); err != nil {
			t.Errorf("reply failed: %v", err)
		}
	})

	mockConn := newMockConnection()
	go router.HandleConnection(nil, mockConn)

	messageData := map[string]any{
		"id":   "msg-1",
		"path": "/test",
		"data": map[string]string{"request": "test"},
		"meta": map[string]any{"userId": "user123", "timestamp": float64(123456)},
	}
	rawData, err := json.Marshal(messageData)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	mockConn.sendIncoming(&velaros.SocketMessage{
		Type:    velaros.MessageText,
		RawData: rawData,
	})

	response := mockConn.receiveOutgoing(t, time.Second)

	if response.Data == nil {
		t.Fatal("expected response to have Data field set")
	}

	var envelope map[string]any
	if err := json.Unmarshal(response.Data, &envelope); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if envelope["id"] != "msg-1" {
		t.Errorf("expected id=msg-1, got: %v", envelope["id"])
	}
	if data, ok := envelope["data"].(map[string]any); !ok {
		t.Errorf("expected data to be map, got: %T", envelope["data"])
	} else if status, ok := data["status"].(string); !ok || status != "ok" {
		t.Errorf("expected status=ok, got: %v", data["status"])
	}
}

func TestCustomConnection_SocketAssociatedValues(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.Bind("/set-socket-meta", func(ctx *velaros.Context) {
		ctx.SetOnSocket("sessionId", "session-xyz")
		ctx.SetOnSocket("authenticated", true)
		if err := ctx.Send("meta set"); err != nil {
			t.Errorf("reply failed: %v", err)
		}
	})

	router.Bind("/get-socket-meta", func(ctx *velaros.Context) {
		sessionId, ok := ctx.GetFromSocket("sessionId")
		if !ok {
			t.Error("expected sessionId to be set on socket")
			return
		}
		authenticated, ok := ctx.GetFromSocket("authenticated")
		if !ok {
			t.Error("expected authenticated to be set on socket")
			return
		}
		if err := ctx.Send(map[string]any{
			"sessionId":     sessionId,
			"authenticated": authenticated,
		}); err != nil {
			t.Errorf("reply failed: %v", err)
		}
	})

	mockConn := newMockConnection()
	go router.HandleConnection(nil, mockConn)

	msg1, _ := json.Marshal(map[string]any{
		"id":   "msg-1",
		"path": "/set-socket-meta",
	})
	mockConn.sendIncoming(&velaros.SocketMessage{
		Type:    velaros.MessageText,
		RawData: msg1,
	})
	_ = mockConn.receiveOutgoing(t, time.Second)

	msg2, _ := json.Marshal(map[string]any{
		"id":   "msg-2",
		"path": "/get-socket-meta",
	})
	mockConn.sendIncoming(&velaros.SocketMessage{
		Type:    velaros.MessageText,
		RawData: msg2,
	})
	response := mockConn.receiveOutgoing(t, time.Second)

	var envelope map[string]any
	if err := json.Unmarshal(response.Data, &envelope); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got: %T", envelope["data"])
	}

	if sessionId, ok := data["sessionId"].(string); !ok || sessionId != "session-xyz" {
		t.Errorf("expected sessionId=session-xyz, got: %v", data["sessionId"])
	}
	if authenticated, ok := data["authenticated"].(bool); !ok || !authenticated {
		t.Errorf("expected authenticated=true, got: %v", data["authenticated"])
	}
}

func TestWebSocketConnection_WithMetaField(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.Bind("/echo", func(ctx *velaros.Context) {
		var req struct {
			Message string `json:"message"`
		}
		if err := ctx.ReceiveInto(&req); err != nil {
			t.Errorf("unmarshal failed: %v", err)
			return
		}

		requestId, ok := ctx.Meta("requestId")
		if !ok {
			t.Error("expected requestId in meta")
			return
		}

		if err := ctx.Send(map[string]any{
			"echo":      req.Message,
			"requestId": requestId,
		}); err != nil {
			t.Errorf("reply failed: %v", err)
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

	msg := map[string]any{
		"id":   "test-1",
		"path": "/echo",
		"data": map[string]string{"message": "hello"},
		"meta": map[string]any{"requestId": "req-123"},
	}
	msgBytes, _ := json.Marshal(msg)
	if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
		t.Fatal(err)
	}

	_, responseBytes, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var response map[string]any
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if response["id"] != "test-1" {
		t.Errorf("expected id=test-1, got: %v", response["id"])
	}

	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got: %T", response["data"])
	}
	if echo, ok := data["echo"].(string); !ok || echo != "hello" {
		t.Errorf("expected echo=hello, got: %v", data["echo"])
	}
	if requestId, ok := data["requestId"].(string); !ok || requestId != "req-123" {
		t.Errorf("expected requestId=req-123, got: %v", data["requestId"])
	}
}
