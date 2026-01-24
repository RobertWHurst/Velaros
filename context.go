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

// ErrContextFreed is returned when attempting to use a Context after its handler
// has returned. Context objects are pooled and reused, so handlers must block
// until all operations using the context are complete.
var ErrContextFreed = errors.New("context cannot be used after handler returns - handlers must block until all operations complete")

// Context represents the context of a WebSocket message being processed. It
// provides access to the message data, connection state, and methods for
// sending responses. Context implements Go's context.Context interface for
// integration with standard context-aware code.
//
// Context objects are pooled and reused for performance. Each incoming message
// gets its own Context instance from the pool, which is returned to the pool
// after the handler chain completes.
//
// Key capabilities:
//   - Message access: Path(), Data(), MessageID(), Params()
//   - Storage: Set/Get for per-message storage, SetOnSocket/GetFromSocket for per-connection storage
//   - Communication: Send(), Receive()
//   - Control flow: Next() to continue the handler chain
//   - Lifecycle: Close(), CloseWithStatus() to terminate the connection
//   - Error handling: Error, ErrorStack fields for panic recovery
type Context struct {
	// mu protects all mutable fields for concurrent access. Use RLock for reads,
	// Lock for writes. Fields marked "immutable" are set once during creation
	// and don't need locking.
	mu sync.RWMutex

	// Immutable after creation
	parentContext *Context
	socket        *Socket
	ctx           context.Context
	cancelCtx     context.CancelFunc

	// Mutable - protected by mu
	message       *InboundMessage
	params        MessageParams
	messageType   websocket.MessageType

	messageUnmarshaler func(message *InboundMessage, into any) error
	messageMarshaller  func(message *OutboundMessage) ([]byte, error)

	currentHandlerNode        *HandlerNode
	currentHandlerNodeMatches bool
	associatedValues          map[string]any
	currentHandlerIndex       int

	currentHandler any

	interceptorChan  chan *InboundMessage
	ownsInterceptor  bool
	firstReceiveDone bool

	// Error holds any error that occurred during handler execution, including
	// errors from panics (automatically recovered). When Error is set, subsequent
	// handlers in the chain are skipped. Error-handling middleware should call
	// Next() first, then check this field after handlers execute.
	Error error

	// ErrorStack contains the stack trace if Error was set by a panic. This is
	// useful for debugging and logging. The stack trace has the framework's
	// internal frames removed for clarity.
	ErrorStack string
}

var _ context.Context = &Context{}

// NewContext creates a new Context with the given socket, message, and handlers.
// The router creates contexts automatically when handling WebSocket connections,
// so most users should not call this directly.
func NewContext(sender *Socket, message *InboundMessage, handlers ...any) *Context {
	return NewContextWithNode(sender, message, &HandlerNode{Handlers: handlers})
}

// NewContextWithNode creates a Context with a specific handler node. Most users
// should use NewContext instead.
func NewContextWithNode(socket *Socket, message *InboundMessage, firstHandlerNode *HandlerNode) *Context {
	return NewContextWithNodeAndMessageType(socket, message, firstHandlerNode, websocket.MessageText)
}

// NewContextWithNodeAndMessageType creates a Context with a specific handler node
// and WebSocket message type. This is for internal use by the router. Users should
// not call this directly.
func NewContextWithNodeAndMessageType(socket *Socket, message *InboundMessage, firstHandlerNode *HandlerNode, messageType websocket.MessageType) *Context {
	ctx := contextFromPool()
	ctx.ctx, ctx.cancelCtx = context.WithCancel(socket)

	ctx.socket = socket
	ctx.message = message
	ctx.messageType = messageType

	if message.ID == "" {
		message.ID = uuid.NewString()
	}

	ctx.currentHandlerNode = firstHandlerNode

	return ctx
}

// NewSubContextWithNode creates a sub-context for nested handler chains. This is
// used internally by the router when mounting routers within routers. Users should
// not call this directly.
func NewSubContextWithNode(ctx *Context, firstHandlerNode *HandlerNode) *Context {
	subCtx := contextFromPool()
	subCtx.ctx, subCtx.cancelCtx = context.WithCancel(ctx)

	subCtx.parentContext = ctx

	subCtx.socket = ctx.socket

	// Get a fresh message from the pool to prevent sub-contexts from sharing
	// message references with parent contexts. If the parent is freed and its
	// message recycled while the sub-context is still alive (e.g., in a blocking
	// handler), the sub-context would see the recycled message's fields.
	subMsg := inboundMessageFromPool()
	subMsg.hasSetID = ctx.message.hasSetID
	subMsg.hasSetPath = ctx.message.hasSetPath
	subMsg.ID = ctx.message.ID
	subMsg.Path = ctx.message.Path
	subMsg.RawData = ctx.message.RawData
	subMsg.Data = ctx.message.Data
	subMsg.Meta = ctx.message.Meta
	subCtx.message = subMsg

	subCtx.messageType = ctx.messageType

	for k, v := range ctx.params {
		subCtx.params[k] = v
	}

	subCtx.Error = ctx.Error
	subCtx.ErrorStack = ctx.ErrorStack

	subCtx.messageUnmarshaler = ctx.messageUnmarshaler
	subCtx.messageMarshaller = ctx.messageMarshaller

	for k, v := range ctx.associatedValues {
		subCtx.associatedValues[k] = v
	}

	subCtx.interceptorChan = ctx.interceptorChan
	subCtx.ownsInterceptor = false // Sub-contexts never own interceptors
	subCtx.firstReceiveDone = ctx.firstReceiveDone

	subCtx.currentHandlerNode = firstHandlerNode

	return subCtx
}

var contextPool = sync.Pool{
	New: func() any {
		return &Context{
			params:           MessageParams{},
			associatedValues: map[string]any{},
		}
	},
}

func contextFromPool() *Context {
	return contextPool.Get().(*Context)
}

func (c *Context) free() {
	c.cancelCtx()

	// Clean up interceptor before freeing message (to avoid race on message.ID)
	// Only clean up if this context owns the interceptor (created it)
	if c.interceptorChan != nil && c.ownsInterceptor {
		if c.socket != nil && c.message != nil {
			c.socket.RemoveInterceptor(c.message.ID)
		}
		// Drain and free any buffered messages
		for len(c.interceptorChan) > 0 {
			msg := <-c.interceptorChan
			msg.free()
		}
	}
	c.interceptorChan = nil
	c.ownsInterceptor = false

	if c.message != nil {
		c.message.free()
	}

	c.parentContext = nil

	c.socket = nil
	c.message = nil
	c.messageType = 0

	for k := range c.params {
		delete(c.params, k)
	}

	c.Error = nil
	c.ErrorStack = ""

	c.messageUnmarshaler = nil
	c.messageMarshaller = nil

	c.currentHandlerNode = nil
	c.currentHandlerNodeMatches = false
	c.currentHandlerIndex = 0
	c.currentHandler = nil

	for k := range c.associatedValues {
		delete(c.associatedValues, k)
	}

	c.firstReceiveDone = false

	contextPool.Put(c)
}

func (c *Context) tryUpdateParent() {
	if c.parentContext == nil {
		return
	}

	c.parentContext.Error = c.Error
	c.parentContext.ErrorStack = c.ErrorStack

	for k, v := range c.associatedValues {
		c.parentContext.associatedValues[k] = v
	}
}

// SetOnSocket stores a value at the socket/connection level. Values stored
// here persist across all messages for the duration of the WebSocket connection.
// This is thread-safe and can be called from handlers processing different messages
// concurrently on the same connection.
//
// Common uses: authentication state, user info, connection metadata.
//
// Example:
//
//	ctx.SetOnSocket("userID", "12345")
//	ctx.SetOnSocket("authenticated", true)
func (c *Context) SetOnSocket(key string, value any) {
	if c.socket == nil {
		return
	}
	c.socket.Set(key, value)
}

// GetFromSocket retrieves a value stored at the socket/connection level.
// Returns the value and true if found, or nil and false if not found.
// This is thread-safe and can be called from handlers processing different
// messages concurrently on the same connection.
func (c *Context) GetFromSocket(key string) (any, bool) {
	if c.socket == nil {
		return nil, false
	}
	return c.socket.Get(key)
}

// MustGetFromSocket retrieves a value stored at the socket/connection level.
// Panics if the key is not found. Use this when the value is expected to exist.
func (c *Context) MustGetFromSocket(key string) any {
	if c.socket == nil {
		panic(ErrContextFreed)
	}
	return c.socket.MustGet(key)
}

// DeleteFromSocket removes a value stored at the socket/connection level. This is
// thread-safe and can be called from handlers processing different messages concurrently
// on the same connection.
func (c *Context) DeleteFromSocket(key string) {
	if c.socket == nil {
		return
	}
	c.socket.Delete(key)
}

// Set stores a value at the message level. Values stored here only exist for
// the duration of processing this specific message. This is useful for passing
// data between middleware and handlers in the same message's handler chain.
// This method is safe for concurrent use.
//
// Example:
//
//	ctx.Set("startTime", time.Now())
//	ctx.Set("requestID", uuid.New())
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	c.associatedValues[key] = value
	c.mu.Unlock()
}

// Get retrieves a value stored at the message level. Returns the value and
// true if found, or nil and false if not found. Values are only accessible
// within the same message's handler chain.
// This method is safe for concurrent use.
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	v, ok := c.associatedValues[key]
	c.mu.RUnlock()
	return v, ok
}

// MustGet retrieves a value stored at the message level. Panics if the key
// is not found. Use this when the value is expected to exist.
// This method is safe for concurrent use.
func (c *Context) MustGet(key string) any {
	c.mu.RLock()
	v, ok := c.associatedValues[key]
	c.mu.RUnlock()
	if !ok {
		panic("key not found")
	}
	return v
}

// Delete removes a value stored at the message level with Set. Values are only
// accessible within the same message's handler chain.
// This method is safe for concurrent use.
func (c *Context) Delete(key string) {
	c.mu.Lock()
	delete(c.associatedValues, key)
	c.mu.Unlock()
}

// SocketID returns the unique identifier for this WebSocket connection.
// The ID is generated when the connection is established and remains constant
// for the connection's lifetime.
func (c *Context) SocketID() string {
	if c.socket == nil {
		return ""
	}
	return c.socket.id
}

// MessageID returns the ID of the current message. Message IDs are used for
// request/reply correlation. If the incoming message didn't have an ID, one
// is automatically generated.
// This method is safe for concurrent use.
func (c *Context) MessageID() string {
	c.mu.RLock()
	id := c.message.ID
	c.mu.RUnlock()
	return id
}

// RawData returns the raw byte data of the current message before any middleware
// processing. This is the unmodified data as received from the WebSocket.
// This method is safe for concurrent use.
func (c *Context) RawData() []byte {
	c.mu.RLock()
	data := c.message.RawData
	c.mu.RUnlock()
	return data
}

// Data returns the processed data of the current message after middleware extraction.
// Middleware may extract this from a nested 'data' field or transform RawData. For the
// original unmodified data, use RawData().
// This method is safe for concurrent use.
func (c *Context) Data() []byte {
	c.mu.RLock()
	data := c.message.Data
	c.mu.RUnlock()
	return data
}

// Path returns the path of the current message. The path is used for routing
// and pattern matching. Middleware can modify the path via SetMessagePath().
// This method is safe for concurrent use.
func (c *Context) Path() string {
	c.mu.RLock()
	path := c.message.Path
	c.mu.RUnlock()
	return path
}

// MessageType returns the WebSocket message type of the current message (MessageText
// or MessageBinary). This is determined by the underlying WebSocket protocol, not the
// message content.
// This method is safe for concurrent use.
func (c *Context) MessageType() MessageType {
	c.mu.RLock()
	mt := c.messageType
	c.mu.RUnlock()
	return MessageType(mt)
}

// Headers returns the HTTP headers from the initial WebSocket upgrade request.
// These headers persist for the lifetime of the connection and are useful for
// accessing things like authentication tokens, cookies, or custom headers sent
// during the handshake.
//
// Example:
//
//	token := ctx.Headers().Get("Authorization")
//	cookie, _ := (&http.Request{Header: ctx.Headers()}).Cookie("session")
func (c *Context) Headers() http.Header {
	if c.socket == nil {
		return nil
	}
	return c.socket.Headers()
}

// RemoteAddr returns the remote address of the client for the WebSocket
// connection.
func (c *Context) RemoteAddr() string {
	if c.socket == nil {
		return ""
	}
	return c.socket.RemoteAddr()
}

// Params returns a copy of the parameters extracted from the message path based on the
// matched route pattern. Parameters are defined in patterns using :name syntax.
// This method is safe for concurrent use.
//
// Example pattern: "/users/:id"
// Example path: "/users/123"
// Result: Params().Get("id") returns "123"
func (c *Context) Params() MessageParams {
	c.mu.RLock()
	// Return a copy to prevent concurrent map access
	paramsCopy := make(MessageParams, len(c.params))
	for k, v := range c.params {
		paramsCopy[k] = v
	}
	c.mu.RUnlock()
	return paramsCopy
}

// SetMessageUnmarshaler sets the function used to unmarshal incoming message
// data. This is typically called by middleware (like JSON middleware) to provide
// a deserialization strategy for the rest of the handler chain.
// This method is safe for concurrent use.
func (c *Context) SetMessageUnmarshaler(unmarshaler func(message *InboundMessage, into any) error) {
	c.mu.Lock()
	c.messageUnmarshaler = unmarshaler
	c.mu.Unlock()
}

// SetMessageMarshaller sets the function used to marshal outgoing message data.
// This is typically called by middleware (like JSON middleware) to provide a
// serialization strategy for Send() and Request() methods.
// This method is safe for concurrent use.
func (c *Context) SetMessageMarshaller(marshaller func(message *OutboundMessage) ([]byte, error)) {
	c.mu.Lock()
	c.messageMarshaller = marshaller
	c.mu.Unlock()
}

// SetMessageID changes the ID of the current message. This affects routing
// behavior - the framework will attempt to deliver this message to any registered
// interceptor (from Request/RequestInto calls) matching this ID.
// This method is safe for concurrent use.
//
// This is primarily used by middleware to implement custom request/reply patterns.
func (c *Context) SetMessageID(id string) {
	c.mu.Lock()
	c.message.ID = id
	c.message.hasSetID = true
	c.mu.Unlock()
}

// SetMessagePath changes the path of the current message. This affects routing
// behavior - after the path is changed, the framework will re-evaluate route
// patterns to find matching handlers.
// This method is safe for concurrent use.
//
// This is useful for middleware that needs to rewrite or normalize paths before
// they reach handlers.
func (c *Context) SetMessagePath(path string) {
	c.mu.Lock()
	c.message.Path = path
	c.message.hasSetPath = true
	c.mu.Unlock()
}

// SetMessageRawData replaces the raw message data. This is useful for middleware
// that needs to transform or filter the message payload before it reaches handlers.
// This method is safe for concurrent use.
func (c *Context) SetMessageRawData(rawData []byte) {
	c.mu.Lock()
	c.message.RawData = rawData
	c.mu.Unlock()
}

// SetMessageData replaces the raw message data. This is useful for middleware
// that needs to transform or filter the message payload before it reaches handlers.
// This method is safe for concurrent use.
func (c *Context) SetMessageData(data []byte) {
	c.mu.Lock()
	c.message.Data = data
	c.mu.Unlock()
}

// SetMessageMeta sets the metadata map for the current message. Metadata can
// be used to pass authentication tokens, tracing IDs, or other contextual
// information alongside message data.
// This method is safe for concurrent use.
func (c *Context) SetMessageMeta(meta map[string]any) {
	c.mu.Lock()
	c.message.Meta = meta
	c.mu.Unlock()
}

// Meta retrieves a value from the message metadata by key.
// Returns the value and true if the key exists, or nil and false otherwise.
// Metadata can be used to pass authentication tokens, tracing IDs, or other
// contextual information alongside message data.
// This method is safe for concurrent use.
//
// Example:
//
//	userId, ok := ctx.Meta("userId")
//	if ok {
//	    log.Printf("Request from user: %v", userId)
//	}
func (c *Context) Meta(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.message == nil || c.message.Meta == nil {
		return nil, false
	}
	value, ok := c.message.Meta[key]
	return value, ok
}

// Send sends a message to the client. If the current message has an ID, the
// response uses the same ID, enabling the client to correlate the response
// with their request. If there's no ID, the message is sent without one.
// This method is safe for concurrent use.
//
// Example:
//
//	var req GetUserRequest
//	ctx.ReceiveInto(&req)
//	user := getUser(req.UserID)
//	ctx.Send(GetUserResponse{User: user})
func (c *Context) Send(data any) error {
	if c.socket == nil {
		return ErrContextFreed
	}
	c.mu.RLock()
	messageID := c.message.ID
	messageType := c.messageType
	c.mu.RUnlock()

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:   messageID,
		Data: data,
	})
	if err != nil {
		return err
	}
	return c.socket.Send(messageType, msgBuf)
}

// Request sends a message to the client and waits for a response, returning the raw
// response data. This is a convenience method that combines Send() and Receive().
// Blocks until a response arrives, the connection closes, or the context is cancelled.
//
// Returns the raw response data and an error if sending fails, the connection closes,
// or the context is cancelled.
//
// Example usage:
//
//	data, err := ctx.Request(query)
//	if err != nil {
//	    return err
//	}
//	process(data)
func (c *Context) Request(data any) ([]byte, error) {
	return c.RequestWithTimeout(data, 0) // 0 = no timeout
}

// RequestWithTimeout is like Request but allows specifying a custom timeout duration.
// A timeout of 0 means no timeout (wait indefinitely).
func (c *Context) RequestWithTimeout(data any, timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		return c.RequestWithContext(c.ctx, data)
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.RequestWithContext(ctx, data)
}

// RequestWithContext is like Request but accepts a custom context for cancellation.
func (c *Context) RequestWithContext(ctx context.Context, data any) ([]byte, error) {
	// If this is the first receive, skip the trigger message
	c.mu.Lock()
	c.firstReceiveDone = true
	c.mu.Unlock()

	if err := c.Send(data); err != nil {
		return nil, err
	}
	return c.ReceiveWithContext(ctx)
}

// RequestInto sends a message to the client and waits for a response, unmarshalling
// the response into the provided value. This is a convenience method that combines
// Send() and ReceiveInto().
// Blocks until a response arrives, the connection closes, or the context is cancelled.
//
// Returns an error if sending fails, the connection closes, context is cancelled,
// or unmarshalling fails.
//
// Example usage:
//
//	var response QueryResponse
//	if err := ctx.RequestInto(query, &response); err != nil {
//	    return err
//	}
//	process(response)
func (c *Context) RequestInto(data any, into any) error {
	return c.RequestIntoWithTimeout(data, into, 0) // 0 = no timeout
}

// RequestIntoWithTimeout is like RequestInto but allows specifying a custom timeout duration.
// A timeout of 0 means no timeout (wait indefinitely).
func (c *Context) RequestIntoWithTimeout(data any, into any, timeout time.Duration) error {
	if timeout == 0 {
		return c.RequestIntoWithContext(c.ctx, data, into)
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.RequestIntoWithContext(ctx, data, into)
}

// RequestIntoWithContext is like RequestInto but accepts a custom context for cancellation.
func (c *Context) RequestIntoWithContext(ctx context.Context, data any, into any) error {
	// If this is the first receive, skip the trigger message
	c.mu.Lock()
	c.firstReceiveDone = true
	c.mu.Unlock()

	if err := c.Send(data); err != nil {
		return err
	}
	return c.ReceiveIntoWithContext(ctx, into)
}

// Receive waits for the next message from the client with the context's message ID
// and returns the raw data. Blocks until a message arrives, the connection closes,
// or the context is cancelled.
//
// An interceptor is automatically set up the first time a message with an ID arrives.
// Subsequent messages with the same ID are intercepted and queued for the handler.
//
// Returns the raw message data and an error if the connection closes or context is cancelled.
//
// Example usage:
//
//	ctx.Send(greeting)
//	for {
//	    data, err := ctx.Receive()
//	    if err != nil {
//	        return
//	    }
//	    ctx.Send(process(data))
//	}
func (c *Context) Receive() ([]byte, error) {
	return c.ReceiveWithTimeout(0) // 0 = no timeout
}

// ReceiveWithTimeout is like Receive but allows specifying a custom timeout duration.
// A timeout of 0 means no timeout (wait indefinitely).
func (c *Context) ReceiveWithTimeout(timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		return c.ReceiveWithContext(c.ctx)
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.ReceiveWithContext(ctx)
}

// ReceiveWithContext is like Receive but accepts a custom context for cancellation.
func (c *Context) ReceiveWithContext(ctx context.Context) ([]byte, error) {
	if c.socket == nil {
		return nil, ErrContextFreed
	}

	// First receive: if trigger message has data, return it; otherwise skip to next message
	c.mu.Lock()
	isFirstReceive := !c.firstReceiveDone
	if isFirstReceive {
		c.firstReceiveDone = true
	}
	messageData := c.message.Data
	interceptorChan := c.interceptorChan
	c.mu.Unlock()

	if isFirstReceive {
		if len(messageData) > 0 {
			return messageData, nil
		}
		// Empty trigger message - fall through to receive next message
	}

	if interceptorChan == nil {
		return nil, errors.New("cannot receive subsequent messages: client must send message with an 'id' field for multi-message conversations")
	}

	select {
	case message := <-interceptorChan:
		data := message.Data
		message.free()
		return data, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("receive cancelled: %w", ctx.Err())
	}
}

// ReceiveInto waits for the next message from the client with the context's message ID
// and unmarshals it into the provided value. Blocks until a message arrives, the
// connection closes, or the context is cancelled.
//
// An interceptor is automatically set up the first time a message with an ID arrives.
// Subsequent messages with the same ID are intercepted and queued for the handler.
//
// Returns an error if the connection closes, context is cancelled, or unmarshalling fails.
//
// Example usage:
//
//	ctx.Send(greeting)
//	for {
//	    var msg UserMessage
//	    if err := ctx.ReceiveInto(&msg); err != nil {
//	        return
//	    }
//	    ctx.Send(process(msg))
//	}
func (c *Context) ReceiveInto(into any) error {
	return c.ReceiveIntoWithTimeout(into, 0) // 0 = no timeout
}

// ReceiveIntoWithTimeout is like ReceiveInto but allows specifying a custom timeout duration.
// A timeout of 0 means no timeout (wait indefinitely).
func (c *Context) ReceiveIntoWithTimeout(into any, timeout time.Duration) error {
	if timeout == 0 {
		return c.ReceiveIntoWithContext(c.ctx, into)
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()
	return c.ReceiveIntoWithContext(ctx, into)
}

// ReceiveIntoWithContext is like ReceiveInto but accepts a custom context for cancellation.
func (c *Context) ReceiveIntoWithContext(ctx context.Context, into any) error {
	if c.socket == nil {
		return ErrContextFreed
	}

	// First receive: if trigger message has data, return it; otherwise skip to next message
	c.mu.Lock()
	isFirstReceive := !c.firstReceiveDone
	if isFirstReceive {
		c.firstReceiveDone = true
	}
	message := c.message
	interceptorChan := c.interceptorChan
	c.mu.Unlock()

	if isFirstReceive {
		if len(message.Data) > 0 {
			return c.unmarshalInboundMessage(message, into)
		}
		// Empty trigger message - fall through to receive next message
	}

	if interceptorChan == nil {
		return errors.New("cannot receive subsequent messages: client must send message with an 'id' field for multi-message conversations")
	}

	select {
	case message := <-interceptorChan:
		err := c.unmarshalInboundMessage(message, into)
		message.free()
		return err
	case <-ctx.Done():
		return fmt.Errorf("receive cancelled: %w", ctx.Err())
	}
}

// Close closes the WebSocket connection with a normal closure status.
// After calling Close(), the connection will be terminated and no more
// messages can be sent or received. UseClose handlers will still execute
// before the connection is fully closed.
func (c *Context) Close() {
	if c.socket == nil {
		return
	}
	c.socket.Close(StatusNormalClosure, "", ServerCloseSource)
}

// CloseWithStatus closes the WebSocket connection with a specific status code
// and reason string. The status and reason are sent to the client as part of
// the WebSocket close frame. UseClose handlers will still execute before the
// connection is fully closed.
//
// Common status codes:
//   - StatusNormalClosure (1000): Normal closure
//   - StatusGoingAway (1001): Server shutting down
//   - StatusPolicyViolation (1008): Client violated policy
//
// Example:
//
//	ctx.CloseWithStatus(StatusPolicyViolation, "Rate limit exceeded")
func (c *Context) CloseWithStatus(status Status, reason string) {
	if c.socket == nil {
		return
	}
	c.socket.Close(status, reason, ServerCloseSource)
}

// CloseStatus returns the close status code, reason, and source (client or server)
// for the connection. This is useful in UseClose handlers to determine why and how
// the connection was closed.
//
// Returns:
//   - Status: The WebSocket close status code
//   - string: The close reason (may be empty)
//   - CloseSource: Whether the close was initiated by the server or client
//
// Example:
//
//	router.UseClose(func(ctx *velaros.Context) {
//	    status, reason, source := ctx.CloseStatus()
//	    if source == velaros.ClientCloseSource {
//	        log.Printf("Client closed connection: %d %s", status, reason)
//	    }
//	})
func (c *Context) CloseStatus() (Status, string, CloseSource) {
	if c.socket == nil {
		return 0, "", 0
	}
	c.socket.closeMu.Lock()
	defer c.socket.closeMu.Unlock()
	return c.socket.closeStatus, c.socket.closeReason, c.socket.closeStatusSource
}

func (c *Context) unmarshalInboundMessage(message *InboundMessage, into any) error {
	c.mu.RLock()
	unmarshaler := c.messageUnmarshaler
	c.mu.RUnlock()
	if unmarshaler == nil {
		return errors.New("no message unmarshaller set. use SetMessageUnmarshaler or add message parser middleware")
	}
	return unmarshaler(message, into)
}

func (c *Context) marshallOutboundMessage(message *OutboundMessage) ([]byte, error) {
	c.mu.RLock()
	marshaller := c.messageMarshaller
	c.mu.RUnlock()
	if marshaller == nil {
		return nil, errors.New("no message marshaller set. use SetMessageMarshaller() or add data encoder middleware")
	}
	return marshaller(message)
}

// Deadline returns the time when work done on behalf of this context should be
// canceled. Returns ok==false when no deadline is set. Part of the context.Context
// interface.
func (c *Context) Deadline() (time.Time, bool) {
	return c.ctx.Deadline()
}

// Done returns a channel that's closed when work done on behalf of this context
// should be canceled. The channel closes when the WebSocket connection closes or
// Close() is called. Part of the context.Context interface.
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Err returns a non-nil error value after Done is closed. Returns Canceled if the
// context was canceled or DeadlineExceeded if the deadline passed. Part of the
// context.Context interface.
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Value returns the value associated with this context for key. Part of the
// context.Context interface. For Velaros-specific storage, use Get/Set for
// message-level storage or GetFromSocket/SetOnSocket for connection-level storage.
func (c *Context) Value(v any) any {
	return c.ctx.Value(v)
}
