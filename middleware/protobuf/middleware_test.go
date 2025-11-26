package protobuf

import (
	"net/http"
	"testing"

	"github.com/RobertWHurst/velaros"
	"google.golang.org/protobuf/proto"
)

func TestProtobufMiddleware_ValidMessage(t *testing.T) {
	testReq := &TestRequest{
		UserId: 42,
		Name:   "Alice",
	}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Id:   "msg-123",
		Path: "/users/get",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}
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

	var result TestRequest
	if err := ctx.Unmarshal(&result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.UserId != 42 {
		t.Errorf("expected UserId 42, got %d", result.UserId)
	}
	if result.Name != "Alice" {
		t.Errorf("expected Name 'Alice', got %s", result.Name)
	}
}

func TestProtobufMiddleware_MissingID(t *testing.T) {
	testReq := &TestRequest{UserId: 1, Name: "Bob"}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Path: "/test",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}
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

func TestProtobufMiddleware_MissingPath(t *testing.T) {
	testReq := &TestRequest{UserId: 1, Name: "Charlie"}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Id:   "msg-456",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}
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

func TestProtobufMiddleware_InvalidProtobuf(t *testing.T) {
	inboundMsg := &velaros.InboundMessage{RawData: []byte("invalid protobuf \xFF\xFE")}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "velaros-protobuf")
	socket := velaros.NewSocket(&velaros.ConnectionInfo{Headers: headers}, nil)

	nextCalled := false
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {
		nextCalled = true
	})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error == nil {
		t.Fatal("expected error for invalid protobuf")
	}

	if nextCalled {
		t.Error("expected Next() not to be called on error")
	}
}

func TestProtobufMiddleware_NonProtoMessage(t *testing.T) {
	testReq := &TestRequest{UserId: 1, Name: "Dave"}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Path: "/test",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)
	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}

	var result map[string]any
	if err := ctx.Unmarshal(&result); err == nil {
		t.Error("expected error when unmarshaling into non-proto.Message")
	}
}

func TestProtobufMiddleware_ProtocolValidation_Valid(t *testing.T) {
	testReq := &TestRequest{UserId: 1}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{Path: "/test", Data: reqBytes}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "velaros-protobuf")
	socket := velaros.NewSocket(&velaros.ConnectionInfo{Headers: headers}, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}

func TestProtobufMiddleware_ProtocolValidation_Invalid(t *testing.T) {
	testReq := &TestRequest{UserId: 1}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{Path: "/test", Data: reqBytes}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}

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

func TestProtobufMiddleware_ProtocolValidation_Empty(t *testing.T) {
	testReq := &TestRequest{UserId: 1}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{Path: "/test", Data: reqBytes}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}
	socket := velaros.NewSocket(&velaros.ConnectionInfo{}, nil)

	ctx := velaros.NewContext(socket, inboundMsg, func(ctx *velaros.Context) {})

	middleware := Middleware()
	middleware(ctx)

	if ctx.Error != nil {
		t.Fatalf("unexpected error: %v", ctx.Error)
	}
}

func TestProtobufMiddleware_Meta_Incoming(t *testing.T) {
	testReq := &TestRequest{UserId: 42, Name: "Alice"}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Id:   "msg-123",
		Path: "/test",
		Data: reqBytes,
		Meta: map[string][]byte{
			"userId":  []byte(`"user-456"`),
			"traceId": []byte(`"trace-789"`),
			"role":    []byte(`"admin"`),
		},
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}
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


func TestProtobufMiddleware_Meta_MissingMeta(t *testing.T) {
	testReq := &TestRequest{UserId: 1}
	reqBytes, _ := proto.Marshal(testReq)

	env := &Envelope{
		Id:   "msg-123",
		Path: "/test",
		Data: reqBytes,
	}
	envBytes, _ := proto.Marshal(env)

	inboundMsg := &velaros.InboundMessage{RawData: envBytes}
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
