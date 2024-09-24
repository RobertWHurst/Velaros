package scramjet

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

	if c.Error != nil {
		return
	}

	// walk the chain looking for a handler with a pattern that matches the path
	// of the request, or until we reach the end of the chain
	for c.currentHandlerNode != nil {

		// Because handlers can have multiple handler functions or transformers,
		// we may save a matching handler node to the context so that we can
		// continue from the same handler until we have executed all of its
		// handlers and transformers.
		//
		// If we do not have a matching handler node, we will walk the chain
		// until we find a matching handler node.
		if c.matchingHandlerNode == nil {
			for c.currentHandlerNode != nil {
				if c.currentHandlerNode.tryMatch(c) {
					c.matchingHandlerNode = c.currentHandlerNode
					break
				}
				c.currentHandlerNode = c.currentHandlerNode.Next
			}
			if c.matchingHandlerNode == nil {
				break
			}
		}

		// Grab a handler function or transformer from the matching handler node.
		// If there are more than one, we will continue from the same handler node
		// the next time Next is called. We iterate through the handler functions
		// and transformers until we have executed all of them.
		if c.currentHandlerOrTransformerIndex < len(c.matchingHandlerNode.HandlersAndTransformers) {
			c.currentHandlerOrTransformer = c.currentHandlerNode.HandlersAndTransformers[c.currentHandlerOrTransformerIndex]
			c.currentHandlerOrTransformerIndex += 1
			break
		}

		// We only get here if we had a matching handler node, and we have
		// executed all of it's handlers and transformers. We can now clear the
		// matching handler node, and continue to the next handler node.
		c.matchingHandlerNode = nil
		c.currentHandlerNode = c.currentHandlerNode.Next
		c.currentHandlerOrTransformerIndex = 0
		c.currentHandlerOrTransformer = nil
	}

	// If we didn't find a handler function or transformer and we have reached
	// the end of the chain, we can return early.
	if c.currentHandlerOrTransformer == nil {
		return
	}

	// Execute the handler function or transformer. Throw an error if it's not
	// an expected type.
	if currentTransformer, ok := c.currentHandlerOrTransformer.(Transformer); ok {
		execWithCtxRecovery(c, func() {
			currentTransformer.TransformRequest(c)
			c.Next()
			currentTransformer.TransformResponse(c)
		})
	} else if currentHandler, ok := c.currentHandlerOrTransformer.(Handler); ok {
		execWithCtxRecovery(c, func() {
			currentHandler.Handle(c)
		})
	} else if currentHandler, ok := c.currentHandlerOrTransformer.(HandlerFunc); ok {
		execWithCtxRecovery(c, func() {
			currentHandler(c)
		})
	} else if currentHandler, ok := c.currentHandlerOrTransformer.(func(*Context)); ok {
		execWithCtxRecovery(c, func() {
			currentHandler(c)
		})
	} else {
		panic(fmt.Sprintf("Unknown handler type: %s", reflect.TypeOf(c.currentHandlerOrTransformer)))
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
