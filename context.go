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

const DefaultRequestTimeout = 5 * time.Second

type Context struct {
	parentContext *Context

	socket      *socket
	message     *InboundMessage
	params      MessageParams
	messageType websocket.MessageType

	Error           error
	ErrorStack      string
	FinalError      error
	FinalErrorStack string

	currentHandlerNode  *HandlerNode
	matchingHandlerNode *HandlerNode
	currentHandlerIndex int
	currentHandler      any

	associatedValues map[string]any

	messageUnmarshaler func(message *InboundMessage, into any) error
	messageMarshaller  func(message *OutboundMessage) ([]byte, error)

	deadline     *time.Time
	doneHandlers []func()
}

var _ context.Context = &Context{}

func NewContext(sender *socket, message *InboundMessage, handlers ...any) *Context {
	return NewContextWithNode(sender, message, &HandlerNode{Handlers: handlers})
}

func NewContextWithNode(socket *socket, message *InboundMessage, firstHandlerNode *HandlerNode) *Context {
	return NewContextWithNodeAndMessageType(socket, message, firstHandlerNode, websocket.MessageText)
}

func NewContextWithNodeAndMessageType(socket *socket, message *InboundMessage, firstHandlerNode *HandlerNode, messageType websocket.MessageType) *Context {
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

func NewSubContextWithNode(ctx *Context, firstHandlerNode *HandlerNode) *Context {
	subCtx := contextFromPool()

	subCtx.parentContext = ctx

	subCtx.socket = ctx.socket
	subCtx.message = ctx.message
	subCtx.messageType = ctx.messageType

	subCtx.params = ctx.params

	subCtx.Error = ctx.Error
	subCtx.ErrorStack = ctx.ErrorStack
	subCtx.FinalError = ctx.FinalError
	subCtx.FinalErrorStack = ctx.FinalErrorStack

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
	ctx.FinalError = nil
	ctx.FinalErrorStack = ""

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
	contextPool.Put(c)
}

func (c *Context) tryUpdateParent() {
	if c.parentContext == nil {
		return
	}

	c.parentContext.Error = c.Error
	c.parentContext.ErrorStack = c.ErrorStack
	c.parentContext.FinalError = c.FinalError
	c.parentContext.FinalErrorStack = c.FinalErrorStack

	for k, v := range c.associatedValues {
		c.parentContext.associatedValues[k] = v
	}
}

func (c *Context) Next() {
	c.next()
}

func (c *Context) SetOnSocket(key string, value any) {
	c.socket.set(key, value)
}

func (c *Context) GetFromSocket(key string) (any, bool) {
	return c.socket.get(key)
}

func (c *Context) MustGetFromSocket(key string) any {
	return c.socket.mustGet(key)
}

func (c *Context) Set(key string, value any) {
	c.associatedValues[key] = value
}

func (c *Context) Get(key string) (any, bool) {
	v, ok := c.associatedValues[key]
	return v, ok
}

func (c *Context) MustGet(key string) any {
	v, ok := c.associatedValues[key]
	if !ok {
		panic("key not found")
	}
	return v
}

func (c *Context) SocketID() string {
	return c.socket.id
}

func (c *Context) MessageID() string {
	return c.message.ID
}

func (c *Context) Data() []byte {
	return c.message.Data
}

func (c *Context) Path() string {
	return c.message.Path
}

func (c *Context) Headers() http.Header {
	return c.socket.requestHeaders
}

func (c *Context) Params() MessageParams {
	return c.params
}

func (c *Context) Unmarshal(into any) error {
	return c.unmarshalInboundMessage(c.message, into)
}

func (c *Context) SetMessageUnmarshaler(unmarshaler func(message *InboundMessage, into any) error) {
	c.messageUnmarshaler = unmarshaler
}

func (c *Context) SetMessageMarshaller(marshaller func(message *OutboundMessage) ([]byte, error)) {
	c.messageMarshaller = marshaller
}

func (c *Context) SetMessageID(id string) {
	c.message.ID = id
	c.message.hasSetID = true
}

func (c *Context) SetMessagePath(path string) {
	c.message.Path = path
	c.message.hasSetPath = true
}

func (c *Context) SetMessageData(data []byte) {
	c.message.Data = data
}

func (c *Context) Send(data any) error {
	msgBuf, err := c.marshallOutboundMessage(&OutboundMessage{
		Data: data,
	})
	if err != nil {
		return err
	}
	return c.socket.send(c.messageType, msgBuf)
}

func (c *Context) Reply(data any) error {
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

func (c *Context) Request(data any) (any, error) {
	return c.RequestWithTimeout(data, DefaultRequestTimeout)
}

func (c *Context) RequestWithTimeout(data any, timeout time.Duration) (any, error) {
	ctx, cancel := context.WithTimeout(c, timeout)
	defer cancel()
	return c.RequestWithContext(ctx, data)
}

func (c *Context) RequestWithContext(ctx context.Context, data any) (any, error) {
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
		return responseMessage.Data, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
	}
}

func (c *Context) RequestInto(data any, into any) error {
	return c.RequestIntoWithTimeout(data, into, DefaultRequestTimeout)
}

func (c *Context) RequestIntoWithTimeout(data any, into any, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(c, timeout)
	defer cancel()
	return c.RequestIntoWithContext(ctx, data, into)
}

func (c *Context) RequestIntoWithContext(ctx context.Context, data any, into any) error {
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
		return c.unmarshalInboundMessage(responseMessage, into)
	case <-ctx.Done():
		return fmt.Errorf("request cancelled: %w", ctx.Err())
	}
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

// Done added for compatibility with go's context.Context. Alias for
// UntilFinish(). Done is part of the go context.Context interface.
func (c *Context) Done() <-chan struct{} {
	doneChan := make(chan struct{}, 1)
	c.doneHandlers = append(c.doneHandlers, func() {
		doneChan <- struct{}{}
	})
	return doneChan
}

func (c *Context) Err() error {
	return c.FinalError
}

// Value is a noop for compatibility with go's context.Context.
func (c *Context) Value(any) any {
	return nil
}
