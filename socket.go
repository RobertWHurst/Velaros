package velaros

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// MessageType represents the type of a WebSocket message (text or binary). This is
// a type alias for websocket.MessageType from the github.com/coder/websocket package.
type MessageType = websocket.MessageType

const (
	MessageText   MessageType = websocket.MessageText
	MessageBinary MessageType = websocket.MessageBinary
)

// SocketMessage represents a WebSocket message at the transport layer, containing
// the message type, raw data, processed data, and metadata. This is used by
// SocketConnection implementations to pass messages between the WebSocket layer
// and the routing layer.
type SocketMessage struct {
	Type    MessageType
	RawData []byte
	Data    []byte
	Meta    map[string]any
}

// SocketConnection is an interface for WebSocket connection implementations. This
// allows Velaros to work with different WebSocket libraries or custom connection types.
// The framework provides WebSocketConnection for the standard github.com/coder/websocket
// library.
type SocketConnection interface {
	Read(ctx context.Context) (*SocketMessage, error)
	Write(ctx context.Context, msg *SocketMessage) error
	Close(status Status, reason string) error
}

// receiverEntry holds a registered receiver for a specific handler within a node.
// Receivers are used for multi-message conversations: a handler registers a receiver
// via ReceiveInto, and the next same-ID message is delivered to it at exactly this
// node+handlerIndex rather than spawning a new handler instance.
type receiverEntry struct {
	node         *HandlerNode
	handlerIndex int
	ch           chan *InboundMessage
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
//   - Message receiver registration for multi-message conversations
//   - Message-ID locking to serialize same-ID messages through the handler chain
//   - Access to original HTTP upgrade request headers
type Socket struct {
	id                 string
	connectionInfo     *ConnectionInfo
	connection         SocketConnection
	receiversMx        sync.Mutex
	receivers          map[string]*receiverEntry
	messageIDLocksMx   sync.Mutex
	messageIDLocks     map[string]chan struct{}
	associatedValuesMx sync.RWMutex
	associatedValues   map[string]any
	closeMu            sync.Mutex
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
		id:             uuid.NewString(),
		connectionInfo: info,
		connection:     conn,
		receivers:      map[string]*receiverEntry{},
		messageIDLocks: map[string]chan struct{}{},
		associatedValues: map[string]any{},
	}
	s.ctx, s.cancelCtx = context.WithCancel(context.Background())
	return s
}

// ID returns the unique identifier for this socket. The ID is automatically
// generated when the socket is created and remains constant for the connection's lifetime.
func (s *Socket) ID() string {
	return s.id
}

// Headers returns the HTTP headers from the initial WebSocket upgrade request.
// These headers persist for the lifetime of the connection and are useful for
// accessing authentication tokens, cookies, or custom headers sent during the handshake.
func (s *Socket) Headers() http.Header {
	if s.connectionInfo != nil && s.connectionInfo.Headers != nil {
		return s.connectionInfo.Headers
	}
	return http.Header{}
}

// RemoteAddr returns the remote network address of the client. The format depends
// on the underlying connection but is typically 'IP:port'.
func (s *Socket) RemoteAddr() string {
	if s.connectionInfo != nil {
		return s.connectionInfo.RemoteAddr
	}
	return ""
}

// Close marks the socket as closed with the given status code, reason, and source
// (client or server). This is thread-safe and idempotent - subsequent calls have
// no effect. The actual connection close happens after UseClose handlers complete.
func (s *Socket) Close(status Status, reason string, source CloseSource) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}

	s.closed = true
	s.closeStatus = status
	s.closeReason = reason
	s.closeStatusSource = source

	s.receiversMx.Lock()
	for id := range s.receivers {
		close(s.receivers[id].ch)
		delete(s.receivers, id)
	}
	s.receiversMx.Unlock()

	s.messageIDLocksMx.Lock()
	s.messageIDLocks = map[string]chan struct{}{}
	s.messageIDLocksMx.Unlock()

	s.cancelCtx()
}

// IsClosed returns true if the socket has been closed. This is thread-safe and
// can be called from any goroutine.
func (s *Socket) IsClosed() bool {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	return s.closed
}

// Send writes a message to the WebSocket connection with the specified message type
// (MessageText or MessageBinary). This is a low-level method - most users should use
// Context.Send instead.
func (s *Socket) Send(messageType MessageType, data []byte) error {
	return s.SendWithContext(context.Background(), messageType, data)
}

// SendWithContext writes a message to the WebSocket connection with the specified
// message type, using the provided context for cancellation and deadlines.
func (s *Socket) SendWithContext(ctx context.Context, messageType MessageType, data []byte) error {
	return s.connection.Write(ctx, &SocketMessage{
		Type: messageType,
		Data: data,
	})
}

// Set stores a value at the socket/connection level. This is thread-safe and values
// persist for the lifetime of the connection. Use Context.SetOnSocket instead of
// calling this directly.
func (s *Socket) Set(key string, value any) {
	s.associatedValuesMx.Lock()
	s.associatedValues[key] = value
	s.associatedValuesMx.Unlock()
}

// Get retrieves a value stored at the socket/connection level. Returns the value
// and true if found, or nil and false otherwise. This is thread-safe. Use
// Context.GetFromSocket instead of calling this directly.
func (s *Socket) Get(key string) (any, bool) {
	s.associatedValuesMx.RLock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.RUnlock()
	return v, ok
}

// MustGet retrieves a value stored at the socket/connection level. Panics if the
// key is not found. This is thread-safe. Use Context.MustGetFromSocket instead of
// calling this directly.
func (s *Socket) MustGet(key string) any {
	s.associatedValuesMx.RLock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.RUnlock()
	if !ok {
		panic(fmt.Sprintf("key %s not found", key))
	}
	return v
}

// Delete removes a value stored at the socket/connection level. This is thread-safe.
// Use Context.DeleteFromSocket instead of calling this directly.
func (s *Socket) Delete(key string) {
	s.associatedValuesMx.Lock()
	delete(s.associatedValues, key)
	s.associatedValuesMx.Unlock()
}

// HandleNextMessageWithNode reads the next message from the connection and processes
// it through the handler chain starting at the given node. Returns false if the
// connection is closed or an error occurs. This is an internal method used by the router.
func (s *Socket) HandleNextMessageWithNode(node *HandlerNode) bool {
	msg, err := s.connection.Read(s)
	if err != nil {
		closeStatus := websocket.CloseStatus(err)
		if closeStatus != -1 {
			s.Close(Status(closeStatus), "", ClientCloseSource)
			return false
		}
		if errors.Is(err, context.Canceled) {
			s.Close(StatusGoingAway, "", ServerCloseSource)
			return false
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
			s.Close(StatusGoingAway, "", ClientCloseSource)
			return false
		}
		panic(fmt.Errorf("error reading socket message: %w", err))
	}

	go func() {
		inboundMsg := inboundMessageFromPool()
		inboundMsg.RawData = msg.RawData
		inboundMsg.Data = msg.Data
		inboundMsg.Meta = msg.Meta

		ctx := NewContextWithNodeAndMessageType(s, inboundMsg, node, msg.Type)
		ctx.Next()
		if ctx.message != nil && ctx.message.hasSetID {
			s.releaseMessageIDLock(ctx.message.ID)
		}
		ctx.finalize()
	}()

	return true
}

// HandleOpen executes the open lifecycle handlers starting at the given node.
// This is an internal method used by the router when a new connection is established.
func (s *Socket) HandleOpen(node *HandlerNode) {
	openCtx := NewContextWithNode(s, inboundMessageFromPool(), node)
	openCtx.Next()
	openCtx.finalize()
}

// HandleClose executes the close lifecycle handlers starting at the given node.
// This is an internal method used by the router when a connection is closing.
func (s *Socket) HandleClose(node *HandlerNode) {
	closeCtx := NewContextWithNode(s, inboundMessageFromPool(), node)
	closeCtx.Next()
	closeCtx.finalize()
}

// GetReceiverForNode retrieves the receiver channel for a given message ID, but only
// if the registered receiver matches the given node pointer and handler index. This
// ensures same-ID continuation messages are only consumed at the exact handler that
// registered the receiver, allowing middleware on earlier nodes to still run.
func (s *Socket) GetReceiverForNode(id string, node *HandlerNode, handlerIndex int) (chan *InboundMessage, bool) {
	s.receiversMx.Lock()
	defer s.receiversMx.Unlock()

	entry, ok := s.receivers[id]
	if !ok || entry.node != node || entry.handlerIndex != handlerIndex {
		return nil, false
	}
	return entry.ch, true
}

// AddReceiver registers a receiver for a given message ID at a specific handler node
// and index. The next same-ID message will be delivered to this receiver rather than
// invoking the handler again. Receivers are self-consuming: they are removed from
// the map upon delivery.
func (s *Socket) AddReceiver(id string, node *HandlerNode, handlerIndex int, ch chan *InboundMessage) {
	s.receiversMx.Lock()
	defer s.receiversMx.Unlock()

	s.receivers[id] = &receiverEntry{node: node, handlerIndex: handlerIndex, ch: ch}
}

// RemoveReceiver unregisters the receiver for a given message ID. This is a no-op
// if no receiver is registered. Used both for self-consuming delivery cleanup and
// as a safety net in Context.cancelCtx().
func (s *Socket) RemoveReceiver(id string) {
	s.receiversMx.Lock()
	defer s.receiversMx.Unlock()

	delete(s.receivers, id)
}

// acquireMessageIDLock atomically checks whether a message with the given ID is
// already being processed. If not, it creates a new lock channel, stores it, and
// returns (ch, true) — the caller is first and proceeds without blocking. If a lock
// already exists, it returns (ch, false) — the caller must block on <-ch and retry.
func (s *Socket) acquireMessageIDLock(id string) (chan struct{}, bool) {
	s.messageIDLocksMx.Lock()
	defer s.messageIDLocksMx.Unlock()

	if ch, ok := s.messageIDLocks[id]; ok {
		return ch, false
	}
	ch := make(chan struct{})
	s.messageIDLocks[id] = ch
	return ch, true
}

// releaseMessageIDLock releases the lock for the given message ID. It deletes the
// entry from the map and closes the channel, which wakes all goroutines waiting in
// SetMessageID. Those goroutines retry acquireMessageIDLock and race to become the
// next active message. If there are no waiters, close is a no-op and the entry is
// simply gone.
func (s *Socket) releaseMessageIDLock(id string) {
	s.messageIDLocksMx.Lock()
	ch, ok := s.messageIDLocks[id]
	if ok {
		delete(s.messageIDLocks, id)
	}
	s.messageIDLocksMx.Unlock()
	if ok {
		close(ch)
	}
}

// Deadline returns the time when work done on behalf of this socket's context should
// be canceled. Returns ok==false when no deadline is set. Part of the context.Context
// interface.
func (s *Socket) Deadline() (time.Time, bool) {
	return s.ctx.Deadline()
}

// Done returns a channel that's closed when the socket's context should be canceled.
// This closes when the connection closes. Part of the context.Context interface.
func (s *Socket) Done() <-chan struct{} {
	return s.ctx.Done()
}

// Err returns a non-nil error value after Done is closed. Returns Canceled if the
// context was canceled. Part of the context.Context interface.
func (s *Socket) Err() error {
	return s.ctx.Err()
}

// Value returns the value associated with this socket's context for key. Part of the
// context.Context interface.
func (s *Socket) Value(key any) any {
	return s.ctx.Value(key)
}
