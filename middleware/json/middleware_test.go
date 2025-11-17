package json

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/RobertWHurst/velaros"
)

func TestJSONMiddleware_ValidMessage(t *testing.T) {
	msgData := map[string]any{
		"id":   "msg-123",
		"path": "/users/get",
		"data": map[string]any{
			"user_id": 42,
			"name":    "Alice",
		},
	}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	if ctx.MessageID() != "msg-123" {
		t.Errorf("expected MessageID 'msg-123', got '%s'", ctx.MessageID())
	}
	if ctx.Path() != "/users/get" {
		t.Errorf("expected Path '/users/get', got '%s'", ctx.Path())
	}

	if !nextCalled {
		t.Error("expected Next() to be called")
	}

	var result struct {
		UserID int    `json:"user_id"`
		Name   string `json:"name"`
	}
	if err := ctx.Unmarshal(&result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", result.UserID)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Name 'Alice', got %s", result.Name)
	}
}

func TestJSONMiddleware_MissingID(t *testing.T) {
	msgData := map[string]any{
		"path": "/test",
		"data": map[string]string{"msg": "hello"},
	}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	if ctx.Path() != "/test" {
		t.Errorf("expected Path '/test', got '%s'", ctx.Path())
	}
}

func TestJSONMiddleware_MissingPath(t *testing.T) {
	msgData := map[string]any{
		"id":   "msg-456",
		"data": map[string]string{"msg": "hello"},
	}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	if ctx.MessageID() != "msg-456" {
		t.Errorf("expected MessageID 'msg-456', got '%s'", ctx.MessageID())
	}

	if ctx.Path() != "" {
		t.Errorf("expected empty Path, got '%s'", ctx.Path())
	}
}

func TestJSONMiddleware_InvalidJSON(t *testing.T) {
	inboundMsg := &velaros.InboundMessage{Data: []byte("invalid json {{")}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "velaros-json")
	socket := velaros.NewSocket(&velaros.ConnectionInfo{Headers: headers}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if nextCalled {
		t.Error("expected Next() not to be called on error")
	}
}

func TestJSONMiddleware_ProtocolValidation_Valid(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "velaros-json")
	socket := velaros.NewSocket(&velaros.ConnectionInfo{Headers: headers}, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}

func TestJSONMiddleware_ProtocolValidation_Invalid(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "wrong-protocol")
	socket := velaros.NewSocket(&velaros.ConnectionInfo{Headers: headers}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error == nil {
		t.Fatal("expected error for invalid protocol")
	}

	if nextCalled {
		t.Error("expected Next() not to be called on protocol error")
	}
}

func TestJSONMiddleware_ProtocolValidation_Empty(t *testing.T) {
	msgData := map[string]any{"path": "/test"}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}

func TestJSONMiddleware_Marshaller_ErrorType(t *testing.T) {
	outMsg := &velaros.OutboundMessage{
		ID:   "reply-123",
		Data: Error("something went wrong"),
	}

	marshaller := func(message *velaros.OutboundMessage) ([]byte, error) {
		switch v := message.Data.(type) {
		case Error:
			message.Data = M{"error": string(v)}
		}
		envelope := map[string]any{}
		if message.ID != "" {
			envelope["id"] = message.ID
		}
		if message.Data != nil {
			envelope["data"] = message.Data
		}
		return json.Marshal(envelope)
	}

	resultBytes, err := marshaller(outMsg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(resultBytes, &envelope); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if envelope["id"] != "reply-123" {
		t.Errorf("expected id 'reply-123', got %v", envelope["id"])
	}

	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["error"] != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got %v", data["error"])
	}
}

func TestJSONMiddleware_Marshaller_StringType(t *testing.T) {
	outMsg := &velaros.OutboundMessage{
		Data: "hello world",
	}

	marshaller := func(message *velaros.OutboundMessage) ([]byte, error) {
		switch v := message.Data.(type) {
		case string:
			message.Data = M{"message": v}
		}
		envelope := map[string]any{}
		if message.ID != "" {
			envelope["id"] = message.ID
		}
		if message.Data != nil {
			envelope["data"] = message.Data
		}
		return json.Marshal(envelope)
	}

	resultBytes, err := marshaller(outMsg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(resultBytes, &envelope); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["message"] != "hello world" {
		t.Errorf("expected message 'hello world', got %v", data["message"])
	}
}

func TestJSONMiddleware_Marshaller_SingleFieldError(t *testing.T) {
	outMsg := &velaros.OutboundMessage{
		Data: FieldError{Field: "email", Error: "invalid email format"},
	}

	marshaller := func(message *velaros.OutboundMessage) ([]byte, error) {
		switch v := message.Data.(type) {
		case FieldError:
			message.Data = M{
				"error":  "Validation error",
				"fields": genFieldsField([]FieldError{v}),
			}
		}
		envelope := map[string]any{}
		if message.ID != "" {
			envelope["id"] = message.ID
		}
		if message.Data != nil {
			envelope["data"] = message.Data
		}
		return json.Marshal(envelope)
	}

	resultBytes, err := marshaller(outMsg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(resultBytes, &envelope); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["error"] != "Validation error" {
		t.Errorf("expected error 'Validation error', got %v", data["error"])
	}

	fields, ok := data["fields"].([]any)
	if !ok {
		t.Fatal("expected fields to be an array")
	}

	if len(fields) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(fields))
	}

	field0, ok := fields[0].(map[string]any)
	if !ok {
		t.Fatal("expected field to be a map")
	}

	if field0["email"] != "invalid email format" {
		t.Errorf("expected email error 'invalid email format', got %v", field0["email"])
	}
}

func TestJSONMiddleware_Marshaller_MultipleFieldErrors(t *testing.T) {
	outMsg := &velaros.OutboundMessage{
		Data: []FieldError{
			{Field: "email", Error: "invalid email format"},
			{Field: "password", Error: "too short"},
		},
	}

	marshaller := func(message *velaros.OutboundMessage) ([]byte, error) {
		switch v := message.Data.(type) {
		case []FieldError:
			message.Data = M{
				"error":  "Validation error",
				"fields": genFieldsField(v),
			}
		}
		envelope := map[string]any{}
		if message.ID != "" {
			envelope["id"] = message.ID
		}
		if message.Data != nil {
			envelope["data"] = message.Data
		}
		return json.Marshal(envelope)
	}

	resultBytes, err := marshaller(outMsg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(resultBytes, &envelope); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data to be a map")
	}

	if data["error"] != "Validation error" {
		t.Errorf("expected error 'Validation error', got %v", data["error"])
	}

	fields, ok := data["fields"].([]any)
	if !ok {
		t.Fatal("expected fields to be an array")
	}

	if len(fields) != 2 {
		t.Fatalf("expected 2 field errors, got %d", len(fields))
	}

	field0, ok := fields[0].(map[string]any)
	if !ok {
		t.Fatal("expected field to be a map")
	}
	if field0["email"] != "invalid email format" {
		t.Errorf("expected email error 'invalid email format', got %v", field0["email"])
	}

	field1, ok := fields[1].(map[string]any)
	if !ok {
		t.Fatal("expected field to be a map")
	}
	if field1["password"] != "too short" {
		t.Errorf("expected password error 'too short', got %v", field1["password"])
	}
}

func TestJSONMiddleware_Meta_Incoming(t *testing.T) {
	msgData := map[string]any{
		"id":   "msg-123",
		"path": "/test",
		"meta": map[string]any{
			"userId":  "user-456",
			"traceId": "trace-789",
			"role":    "admin",
		},
	}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	if !nextCalled {
		t.Error("expected Next() to be called")
	}

	// Test Meta method
	userId, ok := ctx.Meta("userId")
	if !ok {
		t.Error("expected Meta to find userId")
	}
	if userId != "user-456" {
		t.Errorf("expected userId 'user-456', got %v", userId)
	}

	traceId, ok := ctx.Meta("traceId")
	if !ok {
		t.Error("expected Meta to find traceId")
	}
	if traceId != "trace-789" {
		t.Errorf("expected traceId 'trace-789', got %v", traceId)
	}

	role, ok := ctx.Meta("role")
	if !ok {
		t.Error("expected Meta to find role")
	}
	if role != "admin" {
		t.Errorf("expected role 'admin', got %v", role)
	}

	_, ok = ctx.Meta("nonexistent")
	if ok {
		t.Error("expected Meta to return false for nonexistent key")
	}
}

func TestJSONMiddleware_Meta_MissingMeta(t *testing.T) {
	msgData := map[string]any{
		"id":   "msg-123",
		"path": "/test",
	}
	msgBytes, _ := json.Marshal(msgData)

	inboundMsg := &velaros.InboundMessage{Data: msgBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	_, ok := ctx.Meta("anyKey")
	if ok {
		t.Error("expected Meta to return false when meta is nil")
	}
}
