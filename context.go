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

// DefaultRequestTimeout is the default timeout duration for server-initiated
// requests to the client when using Request() or RequestInto() without explicitly
// specifying a timeout.
const DefaultRequestTimeout = 5 * time.Second

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
//   - Communication: Send(), Reply(), Request(), RequestInto()
//   - Control flow: Next() to continue the handler chain
//   - Lifecycle: Close(), CloseWithStatus() to terminate the connection
//   - Error handling: Error, ErrorStack fields for panic recovery
type Context struct {
	parentContext *Context
	socket        *Socket
	message       *InboundMessage
	params        MessageParams

	currentHandlerNode  *HandlerNode
	matchingHandlerNode *HandlerNode
	currentHandler      any
	associatedValues    map[string]any

	messageUnmarshaler func(message *InboundMessage, into any) error
	messageMarshaller  func(message *OutboundMessage) ([]byte, error)

	deadline *time.Time

	// Error holds any error that occurred during handler execution, including
	// errors from panics (automatically recovered). When Error is set, subsequent
	// handlers in the chain are skipped. Error-handling middleware should call
	// Next() first, then check this field after handlers execute.
	Error error

	// ErrorStack contains the stack trace if Error was set by a panic. This is
	// useful for debugging and logging. The stack trace has the framework's
	// internal frames removed for clarity.
	ErrorStack string

	messageType         websocket.MessageType
	currentHandlerIndex int
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

	if message.ID == "" {
		message.ID = uuid.NewString()
	}

	ctx.socket = socket
	ctx.message = message
	ctx.messageType = messageType

	ctx.currentHandlerNode = firstHandlerNode

	return ctx
}

// NewSubContextWithNode creates a sub-context for nested handler chains. This is
// used internally by the router when mounting routers within routers. Users should
// not call this directly.
func NewSubContextWithNode(ctx *Context, firstHandlerNode *HandlerNode) *Context {
	subCtx := contextFromPool()

	subCtx.parentContext = ctx

	subCtx.socket = ctx.socket
	subCtx.message = ctx.message
	subCtx.messageType = ctx.messageType

	subCtx.params = ctx.params

	subCtx.Error = ctx.Error
	subCtx.ErrorStack = ctx.ErrorStack

	subCtx.messageUnmarshaler = ctx.messageUnmarshaler
	subCtx.messageMarshaller = ctx.messageMarshaller

	for k, v := range ctx.associatedValues {
		subCtx.associatedValues[k] = v
	}

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
	ctx := contextPool.Get().(*Context)

	ctx.parentContext = nil

	ctx.socket = nil
	ctx.message = nil
	ctx.messageType = 0

	for k := range ctx.params {
		delete(ctx.params, k)
	}

	ctx.Error = nil
	ctx.ErrorStack = ""

	ctx.messageUnmarshaler = nil
	ctx.messageMarshaller = nil

	ctx.currentHandlerNode = nil
	ctx.matchingHandlerNode = nil
	ctx.currentHandlerIndex = 0
	ctx.currentHandler = nil

	for k := range ctx.associatedValues {
		delete(ctx.associatedValues, k)
	}

	return ctx
}

func (c *Context) free() {
	if c.message != nil {
		c.message.free()
	}
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

// Next continues execution to the next handler in the chain. Middleware
// should call Next() to pass control to subsequent handlers, then perform
// any post-processing after Next() returns.
//
// If an error is set on the context (via Error field or panic), subsequent
// handlers are skipped. Next() is safe to call multiple times.
func (c *Context) Next() {
	c.next()
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
	c.socket.set(key, value)
}

// GetFromSocket retrieves a value stored at the socket/connection level.
// Returns the value and true if found, or nil and false if not found.
// This is thread-safe and can be called from handlers processing different
// messages concurrently on the same connection.
func (c *Context) GetFromSocket(key string) (any, bool) {
	if c.socket == nil {
		return nil, false
	}
	return c.socket.get(key)
}

// MustGetFromSocket retrieves a value stored at the socket/connection level.
// Panics if the key is not found. Use this when the value is expected to exist.
func (c *Context) MustGetFromSocket(key string) any {
	if c.socket == nil {
		panic("context cannot be used after handler returns - handlers must block until all operations complete")
	}
	return c.socket.mustGet(key)
}

// Set stores a value at the message level. Values stored here only exist for
// the duration of processing this specific message. This is useful for passing
// data between middleware and handlers in the same message's handler chain.
//
// Example:
//
//	ctx.Set("startTime", time.Now())
//	ctx.Set("requestID", uuid.New())
func (c *Context) Set(key string, value any) {
	c.associatedValues[key] = value
}

// Get retrieves a value stored at the message level. Returns the value and
// true if found, or nil and false if not found. Values are only accessible
// within the same message's handler chain.
func (c *Context) Get(key string) (any, bool) {
	v, ok := c.associatedValues[key]
	return v, ok
}

// MustGet retrieves a value stored at the message level. Panics if the key
// is not found. Use this when the value is expected to exist.
func (c *Context) MustGet(key string) any {
	v, ok := c.associatedValues[key]
	if !ok {
		panic("key not found")
	}
	return v
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
func (c *Context) MessageID() string {
	return c.message.ID
}

// Data returns the raw byte data of the current message. This is the message
// data after middleware has potentially modified it via SetMessageData().
func (c *Context) Data() []byte {
	return c.message.Data
}

// Path returns the path of the current message. The path is used for routing
// and pattern matching. Middleware can modify the path via SetMessagePath().
func (c *Context) Path() string {
	return c.message.Path
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
	return c.socket.requestHeaders
}

// Params returns the parameters extracted from the message path based on the
// matched route pattern. Parameters are defined in patterns using :name syntax.
//
// Example pattern: "/users/:id"
// Example path: "/users/123"
// Result: Params().Get("id") returns "123"
func (c *Context) Params() MessageParams {
	return c.params
}

// Unmarshal deserializes the message data into the provided value. The
// deserialization is performed by middleware (typically JSON middleware) via
// the unmarshaler set with SetMessageUnmarshaler(). Returns an error if no
// unmarshaler is set or if deserialization fails.
//
// Example:
//
//	var req ChatMessage
//	if err := ctx.Unmarshal(&req); err != nil {
//	    ctx.Send(ErrorResponse{Error: "invalid message"})
//	    return
//	}
func (c *Context) Unmarshal(into any) error {
	return c.unmarshalInboundMessage(c.message, into)
}

// SetMessageUnmarshaler sets the function used to unmarshal incoming message
// data. This is typically called by middleware (like JSON middleware) to provide
// a deserialization strategy for the rest of the handler chain.
func (c *Context) SetMessageUnmarshaler(unmarshaler func(message *InboundMessage, into any) error) {
	c.messageUnmarshaler = unmarshaler
}

// SetMessageMarshaller sets the function used to marshal outgoing message data.
// This is typically called by middleware (like JSON middleware) to provide a
// serialization strategy for Send(), Reply(), and Request() methods.
func (c *Context) SetMessageMarshaller(marshaller func(message *OutboundMessage) ([]byte, error)) {
	c.messageMarshaller = marshaller
}

// SetMessageID changes the ID of the current message. This affects routing
// behavior - the framework will attempt to deliver this message to any registered
// interceptor (from Request/RequestInto calls) matching this ID.
//
// This is primarily used by middleware to implement custom request/reply patterns.
func (c *Context) SetMessageID(id string) {
	c.message.ID = id
	c.message.hasSetID = true
}

// SetMessagePath changes the path of the current message. This affects routing
// behavior - after the path is changed, the framework will re-evaluate route
// patterns to find matching handlers.
//
// This is useful for middleware that needs to rewrite or normalize paths before
// they reach handlers.
func (c *Context) SetMessagePath(path string) {
	c.message.Path = path
	c.message.hasSetPath = true
}

// SetMessageData replaces the raw message data. This is useful for middleware
// that needs to transform or filter the message payload before it reaches handlers.
func (c *Context) SetMessageData(data []byte) {
	c.message.Data = data
}

// Send sends a message to the client without correlating it to the current
// message. The data is marshalled using the marshaller set by middleware
// (typically JSON middleware). Returns an error if marshalling or sending fails.
//
// Use Send() for notifications and events that aren't responses to a specific
// client request. For responses to client requests, use Reply() instead.
//
// Example:
//
//	ctx.Send(NotificationMessage{Type: "user_online", UserID: "123"})
func (c *Context) Send(data any) error {
	if c.socket == nil {
		return errors.New("context cannot be used after handler returns - handlers must block until all operations complete")
	}
	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		Data: data,
	})
	if err != nil {
		return err
	}
	return c.socket.send(c.messageType, msgBuf)
}

// Reply sends a message to the client in response to the current message.
// The reply includes the same ID as the current message, enabling the client
// to correlate the response with their request. Returns an error if the current
// message has no ID, or if marshalling or sending fails.
//
// Example:
//
//	var req GetUserRequest
//	ctx.Unmarshal(&req)
//	user := getUser(req.UserID)
//	ctx.Reply(GetUserResponse{User: user})
func (c *Context) Reply(data any) error {
	if c.socket == nil {
		return errors.New("context cannot be used after handler returns - handlers must block until all operations complete")
	}
	if c.MessageID() == "" {
		return errors.New("cannot reply to a message without an ID")
	}
	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:   c.MessageID(),
		Data: data,
	})
	if err != nil {
		return err
	}
	return c.socket.send(c.messageType, msgBuf)
}

// Request sends a message to the client and waits for a response. This enables
// server-initiated request/reply patterns. The request uses a generated ID and
// waits for a matching response from the client. Uses DefaultRequestTimeout (5s).
//
// Returns the raw response data (as []byte) or an error if the request times out,
// fails to send, or the connection closes. For automatic deserialization, use
// RequestInto() instead.
//
// Example:
//
//	response, err := ctx.Request(ServerRequest{Action: "confirm"})
//	if err != nil {
//	    log.Printf("Client didn't respond: %v", err)
//	}
func (c *Context) Request(data any) (any, error) {
	return c.RequestWithTimeout(data, DefaultRequestTimeout)
}

// RequestWithTimeout is like Request but allows specifying a custom timeout duration.
func (c *Context) RequestWithTimeout(data any, timeout time.Duration) (any, error) {
	ctx, cancel := context.WithTimeout(c, timeout)
	defer cancel()
	return c.RequestWithContext(ctx, data)
}

// RequestWithContext is like Request but accepts a custom context for cancellation.
// This allows integration with Go's context patterns for request cancellation,
// deadlines, and value propagation.
func (c *Context) RequestWithContext(ctx context.Context, data any) (any, error) {
	if c.socket == nil {
		return nil, errors.New("context cannot be used after handler returns - handlers must block until all operations complete")
	}
	id := uuid.NewString()

	responseMessageChan := make(chan *InboundMessage, 1)
	defer close(responseMessageChan)
	c.socket.addInterceptor(id, responseMessageChan)
	defer c.socket.removeInterceptor(id)

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:   id,
		Data: data,
	})
	if err != nil {
		return nil, err
	}

	if err := c.socket.send(c.messageType, msgBuf); err != nil {
		return nil, err
	}

	select {
	case responseMessage := <-responseMessageChan:
		data := responseMessage.Data
		responseMessage.free()
		return data, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
	}
}

// RequestInto sends a message to the client, waits for a response, and
// unmarshals the response into the provided value. This combines Request()
// with automatic deserialization. Uses DefaultRequestTimeout (5s).
//
// Returns an error if the request times out, fails to send, the connection
// closes, or deserialization fails.
//
// Example:
//
//	var response ConfirmationResponse
//	err := ctx.RequestInto(ServerRequest{Action: "confirm"}, &response)
//	if err != nil {
//	    log.Printf("Request failed: %v", err)
//	    return
//	}
//	if response.Confirmed {
//	    // proceed
//	}
func (c *Context) RequestInto(data any, into any) error {
	return c.RequestIntoWithTimeout(data, into, DefaultRequestTimeout)
}

// RequestIntoWithTimeout is like RequestInto but allows specifying a custom timeout duration.
func (c *Context) RequestIntoWithTimeout(data any, into any, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(c, timeout)
	defer cancel()
	return c.RequestIntoWithContext(ctx, data, into)
}

// RequestIntoWithContext is like RequestInto but accepts a custom context for cancellation.
func (c *Context) RequestIntoWithContext(ctx context.Context, data any, into any) error {
	if c.socket == nil {
		return errors.New("context cannot be used after handler returns - handlers must block until all operations complete")
	}
	id := uuid.NewString()

	responseMessageChan := make(chan *InboundMessage, 1)
	defer close(responseMessageChan)
	c.socket.addInterceptor(id, responseMessageChan)
	defer c.socket.removeInterceptor(id)

	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		ID:   id,
		Data: data,
	})
	if err != nil {
		return err
	}

	if err := c.socket.send(c.messageType, msgBuf); err != nil {
		return err
	}

	select {
	case responseMessage := <-responseMessageChan:
		err := c.unmarshalInboundMessage(responseMessage, into)
		responseMessage.free()
		return err
	case <-ctx.Done():
		return fmt.Errorf("request cancelled: %w", ctx.Err())
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
	c.socket.close(StatusNormalClosure, "", StatusSourceServer)
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
	c.socket.close(status, reason, StatusSourceServer)
}

// CloseStatus returns the close status code, reason, and source (client or server)
// for the connection. This is useful in UseClose handlers to determine why and how
// the connection was closed.
//
// Returns:
//   - Status: The WebSocket close status code
//   - string: The close reason (may be empty)
//   - StatusSource: Whether the close was initiated by the server or client
//
// Example:
//
//	router.UseClose(func(ctx *velaros.Context) {
//	    status, reason, source := ctx.CloseStatus()
//	    if source == velaros.StatusSourceClient {
//	        log.Printf("Client closed connection: %d %s", status, reason)
//	    }
//	})
func (c *Context) CloseStatus() (Status, string, StatusSource) {
	if c.socket == nil {
		return 0, "", 0
	}
	c.socket.closeMx.Lock()
	defer c.socket.closeMx.Unlock()
	return c.socket.closeStatus, c.socket.closeReason, c.socket.closeStatusSource
}

func (c *Context) unmarshalInboundMessage(message *InboundMessage, into any) error {
	if c.messageUnmarshaler == nil {
		return errors.New("no message unmarshaller set. use SetMessageUnmarshaler or add message parser middleware")
	}
	return c.messageUnmarshaler(message, into)
}

func (c *Context) marshallOutboundMessage(message *OutboundMessage) ([]byte, error) {
	if c.messageMarshaller == nil {
		return nil, errors.New("no message marshaller set. use SetMessageMarshaller() or add data encoder middleware")
	}
	return c.messageMarshaller(message)
}

// Deadline returns the deadline of the request. Deadline is part of the go
// context.Context interface.
func (c *Context) Deadline() (time.Time, bool) {
	ok := c.deadline != nil
	deadline := time.Time{}
	if ok {
		deadline = *c.deadline
	}
	return deadline, ok
}

// Done returns a channel that closes when the socket connection is closed.
// Done is part of the go context.Context interface.
func (c *Context) Done() <-chan struct{} {
	if c.socket == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return c.socket.Done()
}

// Err returns an error if the context is cancelled or the connection is closed.
// This is part of the go context.Context interface. Returns the handler Error
// if set, otherwise returns the socket error (context.Canceled if closed).
func (c *Context) Err() error {
	if c.Error != nil {
		return c.Error
	}
	if c.socket == nil {
		return context.Canceled
	}
	return c.socket.Err()
}

// Value is a no-op for compatibility with go's context.Context interface.
// Use Set/Get for per-message values or SetOnSocket/GetFromSocket for
// per-connection values instead.
func (c *Context) Value(any) any {
	return nil
}
