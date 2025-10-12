// Package velaros provides a lightweight, flexible WebSocket framework for Go.
//
// Velaros enables building real-time WebSocket applications with pattern-based
// message routing, composable middleware, and bidirectional communication patterns.
//
// # Key Features
//
//   - Pattern-based routing with parameters and wildcards
//   - Composable middleware for authentication, logging, and more
//   - Bidirectional communication with Send, Reply, and Request methods
//   - Context pooling for high performance
//   - Automatic text/binary message type detection
//   - Works with any HTTP router/framework via http.Handler interface
//
// # Quick Start
//
// Create a router, add middleware, and bind handlers to message paths:
//
//	router := velaros.NewRouter()
//	router.Use(json.Middleware())
//
//	router.Bind("/chat/message", func(ctx *velaros.Context) {
//	    var msg ChatMessage
//	    ctx.Unmarshal(&msg)
//	    ctx.Reply(ChatResponse{Status: "received"})
//	})
//
//	http.ListenAndServe(":8080", router)
//
// # Message Format
//
// Message format is determined by middleware. The included JSON middleware
// expects messages with an ID (for request/reply) and path (for routing):
//
//	{
//	  "id": "abc123",
//	  "path": "/chat/message",
//	  "username": "alice",
//	  "text": "Hello!"
//	}
//
// # Routing
//
// Routes support exact paths, named parameters, and wildcards:
//
//	router.Bind("/users/list", handler)           // Exact match
//	router.Bind("/users/:id", handler)            // Named parameter
//	router.Bind("/files/**", handler)             // Wildcard
//
// # Middleware
//
// Middleware executes before handlers and can modify context, perform
// authentication, or short-circuit the chain:
//
//	router.Use(func(ctx *velaros.Context) {
//	    log.Printf("Message: %s", ctx.Path())
//	    ctx.Next()
//	})
//
// # Context Storage
//
// Store values at the message level (per-message) or socket level (per-connection):
//
//	ctx.Set("requestTime", time.Now())              // Per-message
//	ctx.SetOnSocket("username", "alice")            // Per-connection
//
// For more examples and documentation, see https://github.com/RobertWHurst/velaros
package velaros
