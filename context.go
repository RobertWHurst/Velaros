package scramjet

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

type Context struct {
	parentContext *Context

	id      string
	socket  *Socket
	message *InboundMessage
	path    string
	params  MessageParams

	Error           error
	ErrorStack      string
	FinalError      error
	FinalErrorStack string

	currentHandlerNode               *HandlerNode
	matchingHandlerNode              *HandlerNode
	currentHandlerOrTransformerIndex int
	currentHandlerOrTransformer      any

	associatedValues map[string]any

	messageDataUnmarshaler func(into any) error
	messageDataMarshaller  func(from any) ([]byte, error)
}

func NewContext(sender *Socket, message *InboundMessage, handlers ...any) *Context {
	return NewContextWithNode(sender, message, &HandlerNode{HandlersAndTransformers: handlers})
}

func NewContextWithNode(socket *Socket, message *InboundMessage, firstHandlerNode *HandlerNode) *Context {
	ctx := contextFromPool()

	ctx.socket = socket
	ctx.message = message
	ctx.id = message.ID
	ctx.path = message.Path

	if ctx.id == "" {
		ctx.id = uuid.NewString()
	}

	ctx.currentHandlerNode = firstHandlerNode

	return ctx
}

func NewSubContextWithNode(ctx *Context, firstHandlerNode *HandlerNode) *Context {
	subCtx := contextFromPool()

	subCtx.parentContext = ctx

	subCtx.socket = ctx.socket
	subCtx.message = ctx.message

	subCtx.path = ctx.path
	subCtx.params = ctx.params

	subCtx.Error = ctx.Error
	subCtx.ErrorStack = ctx.ErrorStack
	subCtx.FinalError = ctx.FinalError
	subCtx.FinalErrorStack = ctx.FinalErrorStack

	subCtx.messageDataUnmarshaler = ctx.messageDataUnmarshaler
	subCtx.messageDataMarshaller = ctx.messageDataMarshaller

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

	ctx.path = ""
	for k := range ctx.params {
		delete(ctx.params, k)
	}

	ctx.Error = nil
	ctx.ErrorStack = ""
	ctx.FinalError = nil
	ctx.FinalErrorStack = ""

	ctx.messageDataUnmarshaler = nil
	ctx.messageDataMarshaller = nil

	ctx.currentHandlerNode = nil
	ctx.matchingHandlerNode = nil
	ctx.currentHandlerOrTransformerIndex = 0
	ctx.currentHandlerOrTransformer = nil

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
	c.socket.Set(key, value)
}

func (c *Context) GetFromSocket(key string) any {
	return c.socket.Get(key)
}

func (c *Context) Set(key string, value any) {
	c.associatedValues[key] = value
}

func (c *Context) Get(key string) any {
	return c.associatedValues[key]
}

func (c *Context) SocketID() string {
	return c.socket.id
}

func (c *Context) MessageID() string {
	return c.id
}

func (c *Context) Path() string {
	return c.path
}

func (c *Context) Params() MessageParams {
	return c.params
}

func (c *Context) UnmarshalMessageData(into any) error {
	if c.messageDataUnmarshaler == nil {
		return errors.New("no message unmarshaller set. use SetMessageDataUnmarshaler or add message parser middleware")
	}
	return c.messageDataUnmarshaler(into)
}

func (c *Context) SetMessageDataUnmarshaler(unmarshaler func(into any) error) {
	c.messageDataUnmarshaler = unmarshaler
}

func (c *Context) SetMessageDataMarshaller(marshaller func(from any) ([]byte, error)) {
	c.messageDataMarshaller = marshaller
}

func (c *Context) WithSocket(socketID string) (*SocketHandle, bool) {
	return c.socket.WithSocket(socketID)
}

func (c *Context) Send(data any) error {
	return c.socket.Send(&OutboundMessage{
		ID:   c.id,
		Data: data,
	})
}

// func (c *Context) Request(data any) (any, error) {
// 	return c.socket.Request(&OutboundMessage{
// 		ID:   c.id,
// 		Data: data,
// 	})
// }
