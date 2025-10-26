package velaros

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
)

// next is called by the public Next method on the context. It can be called
// by handlers to pass the request to the next handler in the chain. Next
// determines which handler is the next matching the request, and executes
// it. For each matching handler node, next will attach params from the path
// to the context. If there are no more handlers, next will do nothing.
func (c *Context) next() {
	// In the case that this is a sub context, we need to update the parent
	// context with the current context's state.
	defer c.tryUpdateParent()

	// For BindClose handlers, we don't want to check if the socket is closed
	// since they're meant to run during the close process
	isCloseHandler := c.currentHandlerNode != nil && c.currentHandlerNode.BindType == CloseBindType
	if c.Error != nil || (!isCloseHandler && c.socket.isClosed()) {
		return
	}

	// if the ID was set, try socket interceptors to see if they want this
	// message
	if c.message.hasSetID {
		interceptorChan, ok := c.socket.getInterceptor(c.message.ID)
		if ok {
			interceptorChan <- c.message
			return
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
