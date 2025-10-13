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
	requestHeaders     http.Header
	connection         *websocket.Conn
	interceptorsMx     sync.Mutex
	interceptors       map[string]chan *InboundMessage
	associatedValuesMx sync.Mutex
	associatedValues   map[string]any
	closeMx            sync.Mutex
	closed             bool
	closeStatus        Status
	closeStatusSource  StatusSource
	closeReason        string
	ctx                context.Context
	cancelCtx          context.CancelFunc
}

var _ context.Context = &Socket{}

// NewSocket creates a new Socket wrapping a WebSocket connection. This is
// primarily for internal use by the router. The socket ID is automatically
// generated and the done channel is initialized.
func NewSocket(requestHeaders http.Header, conn *websocket.Conn) *Socket {
	s := &Socket{
		id:               uuid.NewString(),
		requestHeaders:   requestHeaders,
		connection:       conn,
		interceptors:     map[string]chan *InboundMessage{},
		associatedValues: map[string]any{},
	}
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	return s
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

func (s *Socket) close(status Status, reason string, source StatusSource) {
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

func (s *Socket) isClosed() bool {
	s.closeMx.Lock()
	defer s.closeMx.Unlock()
	return s.closed
}

func (s *Socket) send(messageType websocket.MessageType, data []byte) error {
	return s.connection.Write(context.Background(), messageType, data)
}

func (s *Socket) set(key string, value any) {
	s.associatedValuesMx.Lock()
	s.associatedValues[key] = value
	s.associatedValuesMx.Unlock()
}

func (s *Socket) get(key string) (any, bool) {
	s.associatedValuesMx.Lock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.Unlock()
	return v, ok
}

func (s *Socket) mustGet(key string) any {
	s.associatedValuesMx.Lock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.Unlock()
	if !ok {
		panic(fmt.Sprintf("key %s not found", key))
	}
	return v
}

func (s *Socket) handleNextMessageWithNode(node *HandlerNode) bool {
	msgType, msg, err := s.connection.Read(s)
	if err != nil {
		closeStatus := websocket.CloseStatus(err)
		if closeStatus != -1 {
			s.close(Status(closeStatus), "", StatusSourceClient)
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

func (s *Socket) getInterceptor(id string) (chan *InboundMessage, bool) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	interceptorChan, ok := s.interceptors[id]
	return interceptorChan, ok
}

func (s *Socket) addInterceptor(id string, interceptorChan chan *InboundMessage) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	s.interceptors[id] = interceptorChan
}

func (s *Socket) removeInterceptor(id string) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	delete(s.interceptors, id)
}
