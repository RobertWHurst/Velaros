package velaros

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type MessageType = websocket.MessageType

const (
	MessageText   MessageType = websocket.MessageText
	MessageBinary MessageType = websocket.MessageBinary
)

type SocketConnection interface {
	Read(ctx context.Context) (MessageType, []byte, error)
	Write(ctx context.Context, messageType MessageType, data []byte) error
	Close(status Status, reason string) error
}

// Socket represents a WebSocket connection and manages its lifecycle, message
// interception, and connection-level storage. It implements the context.Context
// interface to support cancellation and deadlines.
//
// Socket is primarily used internally by the router but is exported to allow
// advanced use cases and custom integrations. Most users will interact with
// Socket indirectly through Context methods like SetOnSocket and GetFromSocket.
//
// Key responsibilities:
//   - Connection lifecycle management (open, close, done signaling)
//   - Thread-safe connection-level value storage
//   - Message interception for request/reply correlation
//   - Access to original HTTP upgrade request headers
type Socket struct {
	id                 string
	connectionInfo     *ConnectionInfo
	connection         SocketConnection
	interceptorsMx     sync.Mutex
	interceptors       map[string]chan *InboundMessage
	associatedValuesMx sync.Mutex
	associatedValues   map[string]any
	closeMx            sync.Mutex
	closed             bool
	closeStatus        Status
	closeStatusSource  CloseSource
	closeReason        string
	ctx                context.Context
	cancelCtx          context.CancelFunc
}

var _ context.Context = &Socket{}

// NewSocket creates a new Socket wrapping a WebSocket connection. This is
// primarily for internal use by the router. The socket ID is automatically
// generated and the done channel is initialized.
func NewSocket(info *ConnectionInfo, conn SocketConnection) *Socket {
	s := &Socket{
		id:               uuid.NewString(),
		connectionInfo:   info,
		connection:       conn,
		interceptors:     map[string]chan *InboundMessage{},
		associatedValues: map[string]any{},
	}
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	return s
}

func (s *Socket) ID() string {
	return s.id
}

func (s *Socket) Headers() http.Header {
	if s.connectionInfo != nil && s.connectionInfo.Headers != nil {
		return s.connectionInfo.Headers
	}
	return http.Header{}
}

func (s *Socket) RemoteAddr() string {
	if s.connectionInfo != nil {
		return s.connectionInfo.RemoteAddr
	}
	return ""
}

func (s *Socket) Close(status Status, reason string, source CloseSource) {
	s.closeMx.Lock()
	defer s.closeMx.Unlock()
	if s.closed {
		return
	}

	s.closed = true
	s.closeStatus = status
	s.closeReason = reason
	s.closeStatusSource = source

	s.cancelCtx()
}

func (s *Socket) IsClosed() bool {
	s.closeMx.Lock()
	defer s.closeMx.Unlock()
	return s.closed
}

func (s *Socket) Send(messageType MessageType, data []byte) error {
	return s.connection.Write(context.Background(), messageType, data)
}

func (s *Socket) Set(key string, value any) {
	s.associatedValuesMx.Lock()
	s.associatedValues[key] = value
	s.associatedValuesMx.Unlock()
}

func (s *Socket) Get(key string) (any, bool) {
	s.associatedValuesMx.Lock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.Unlock()
	return v, ok
}

func (s *Socket) MustGet(key string) any {
	s.associatedValuesMx.Lock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.Unlock()
	if !ok {
		panic(fmt.Sprintf("key %s not found", key))
	}
	return v
}

func (s *Socket) Delete(key string) {
	s.associatedValuesMx.Lock()
	delete(s.associatedValues, key)
	s.associatedValuesMx.Unlock()
}

func (s *Socket) HandleNextMessageWithNode(node *HandlerNode) bool {
	msgType, msg, err := s.connection.Read(s)
	if err != nil {
		closeStatus := websocket.CloseStatus(err)
		if closeStatus != -1 {
			s.Close(Status(closeStatus), "", ClientCloseSource)
			return false
		}
		if errors.Is(err, context.Canceled) {
			return false
		}
		panic(fmt.Errorf("error reading socket message: %w", err))
	}

	go func() {
		inboundMsg := inboundMessageFromPool()
		inboundMsg.Data = msg

		ctx := NewContextWithNodeAndMessageType(s, inboundMsg, node, msgType)
		ctx.Next()
		ctx.free()
	}()

	return true
}

func (s *Socket) HandleOpen(node *HandlerNode) {
	openCtx := NewContextWithNode(s, inboundMessageFromPool(), node)
	openCtx.Next()
	openCtx.free()
}

func (s *Socket) HandleClose(node *HandlerNode) {
	closeCtx := NewContextWithNode(s, inboundMessageFromPool(), node)
	closeCtx.Next()
	closeCtx.free()
}

func (s *Socket) GetInterceptor(id string) (chan *InboundMessage, bool) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	interceptorChan, ok := s.interceptors[id]
	return interceptorChan, ok
}

func (s *Socket) AddInterceptor(id string, interceptorChan chan *InboundMessage) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	s.interceptors[id] = interceptorChan
}

func (s *Socket) RemoveInterceptor(id string) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	delete(s.interceptors, id)
}

func (s *Socket) Deadline() (time.Time, bool) {
	return s.ctx.Deadline()
}

func (s *Socket) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *Socket) Err() error {
	return s.ctx.Err()
}

func (s *Socket) Value(key any) any {
	return s.ctx.Value(key)
}
