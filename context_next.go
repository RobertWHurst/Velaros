package velaros

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
)

// Next continues execution to the next handler in the chain. Middleware
// should call Next() to pass control to subsequent handlers, then perform
// any post-processing after Next() returns.
//
// If an error is set on the context (via Error field or panic), subsequent
// handlers are skipped. Next() is safe to call multiple times.
func (c *Context) Next() {
	// In the case that this is a sub context, we need to update the parent
	// context with the current context's state.
	defer c.tryUpdateParent()

	// For BindClose handlers, we don't want to check if the socket is closed
	// since they're meant to run during the close process
	isCloseHandler := c.currentHandlerNode != nil && c.currentHandlerNode.BindType == CloseBindType
	if c.Error != nil || (!isCloseHandler && c.socket.IsClosed()) {
		return
	}

	// Auto-create interceptor if message has an ID. The first message with a given ID
	// creates the interceptor, subsequent messages with the same ID get intercepted.
	if c.message.hasSetID {
		interceptorChan, ok := c.socket.GetInterceptor(c.message.ID)
		if !ok {
			// First message with this ID - create interceptor
			interceptorChan = make(chan *InboundMessage, 1)
			c.interceptorChan = interceptorChan
			c.socket.AddInterceptor(c.message.ID, interceptorChan)
		} else if c.interceptorChan == nil {
			// Subsequent message with this ID - intercept it (unless this context owns the interceptor)
			select {
			case interceptorChan <- c.message:
				return
			case <-c.socket.Done():
				// Socket closed while trying to send - free message and continue
				c.message.free()
				return
			}
		}
		c.message.hasSetID = false
	}

	// if the path was set, and have a matching node, make sure it still matches
	// the path, otherwise clear it and move on to the next node.
	if c.message.hasSetPath {
		if c.currentHandlerNodeMatches && !c.currentHandlerNode.tryMatch(c) {
			c.currentHandlerNode = c.currentHandlerNode.Next
			c.currentHandlerNodeMatches = false
			c.currentHandlerIndex = 0
			c.currentHandler = nil
		}
		c.message.hasSetPath = false
	}

	// walk the chain looking for a handler with a pattern that matches the path
	// of the request, or until we reach the end of the chain
	for c.currentHandlerNode != nil {

		// Because handlers can have multiple handler functions,
		// we may save a matching handler node to the context so that we can
		// continue from the same handler until we have executed all of its
		// handlers.
		//
		// If we do not have a matching handler node, we will walk the chain
		// until we find a matching handler node.
		if !c.currentHandlerNodeMatches {
			for c.currentHandlerNode != nil {
				if c.currentHandlerNode.tryMatch(c) {
					c.currentHandlerNodeMatches = true
					break
				}
				c.currentHandlerNode = c.currentHandlerNode.Next
			}
			if !c.currentHandlerNodeMatches {
				break
			}
		}

		// Grab a handler function from the matching handler node.
		// If there are more than one, we will continue from the same handler node
		// the next time Next is called. We iterate through the handler functions
		// until we have executed all of them.
		if c.currentHandlerIndex < len(c.currentHandlerNode.Handlers) {
			c.currentHandler = c.currentHandlerNode.Handlers[c.currentHandlerIndex]
			c.currentHandlerIndex += 1
			break
		}

		// We only get here if we had a matching handler node, and we have
		// executed all of its handlers. We can now clear the
		// matching handler node, and continue to the next handler node.
		c.currentHandlerNode = c.currentHandlerNode.Next
		c.currentHandlerNodeMatches = false
		c.currentHandlerIndex = 0
		c.currentHandler = nil
	}

	// If we didn't find a handler function and we have reached
	// the end of the chain, we can return early.
	if c.currentHandler == nil {
		return
	}

	// Execute the handler function. Throw an error if it's not
	// an expected type.
	bindType := c.currentHandlerNode.BindType
	if currentHandler, ok := c.currentHandler.(OpenHandler); ok && bindType == OpenBindType {
		execWithCtxRecovery(c, func() {
			currentHandler.HandleOpen(c)
		})
	} else if currentHandler, ok := c.currentHandler.(CloseHandler); ok && bindType == CloseBindType {
		execWithCtxRecovery(c, func() {
			currentHandler.HandleClose(c)
		})
	} else if currentHandler, ok := c.currentHandler.(Handler); ok && bindType == NormalBindType {
		execWithCtxRecovery(c, func() {
			currentHandler.Handle(c)
		})
	} else if currentHandler, ok := c.currentHandler.(HandlerFunc); ok {
		execWithCtxRecovery(c, func() {
			currentHandler(c)
		})
	} else if currentHandler, ok := c.currentHandler.(func(*Context)); ok {
		execWithCtxRecovery(c, func() {
			currentHandler(c)
		})
	} else {
		panic(fmt.Sprintf("Unknown handler type: %s", reflect.TypeOf(c.currentHandler)))
	}

	// Call next automatically for open and close handlers,
	// but only if there was no error and the socket is not closed
	if (bindType == OpenBindType && !c.socket.IsClosed()) || bindType == CloseBindType {
		c.Next()
	}

	// Prevent handlers from calling Next twice
	c.currentHandlerNode = nil
	c.currentHandlerNodeMatches = false
	c.currentHandlerIndex = 0
	c.currentHandler = nil
}

func execWithCtxRecovery(ctx *Context, fn func()) {
	defer func() {
		if maybeErr := recover(); maybeErr != nil {
			if err, ok := maybeErr.(error); ok {
				ctx.Error = err
			} else {
				ctx.Error = fmt.Errorf("%s", maybeErr)
			}

			stack := string(debug.Stack())
			stackLines := strings.Split(stack, "\n")
			ctx.ErrorStack = strings.Join(stackLines[6:], "\n")
		}
	}()
	fn()
}
