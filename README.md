# Velaros

A lightweight, flexible WebSocket framework for Go. Build real-time applications with powerful message routing, bidirectional communication, and composable middleware.

Velaros implements the standard `http.Handler` interface, so it works seamlessly with any Go HTTP router or framework - just mount it on a path like `/ws` and it handles the WebSocket upgrade automatically.

[![Go Reference](https://pkg.go.dev/badge/github.com/RobertWHurst/velaros.svg)](https://pkg.go.dev/github.com/RobertWHurst/velaros)
[![Go Report Card](https://goreportcard.com/badge/github.com/RobertWHurst/velaros)](https://goreportcard.com/report/github.com/RobertWHurst/velaros)

## Features

- 🚀 **High Performance** - Context pooling and efficient message routing
- 🔄 **Bidirectional** - Full duplex communication with Send, Reply, Request, and RequestInto patterns
- 🎯 **Powerful Patterns** - Flexible routing with parameters, wildcards, regex constraints, and modifiers (?, +, *)
- 🔌 **Middleware** - Composable middleware for authentication, logging, and more
- 📦 **Type Detection** - Automatic text/binary message type handling
- ⏱️ **Timeout Control** - Request timeouts and context cancellation for server→client requests
- 🧩 **Extensible** - Simple interfaces for custom handlers and middleware

## Installation

```bash
go get github.com/RobertWHurst/velaros
```

## Quick Start

```go
package main

import (
    "log"
    "net/http"

    "github.com/RobertWHurst/velaros"
    "github.com/RobertWHurst/velaros/middleware/json"
)

type ChatMessage struct {
    Username string `json:"username"`
    Text     string `json:"text"`
}

type ChatResponse struct {
    Status string `json:"status"`
    Text   string `json:"text"`
}

type ErrorResponse struct {
    Error string `json:"error"`
}

func main() {
    router := velaros.NewRouter()

    // Add JSON middleware for automatic encoding/decoding
    router.Use(json.Middleware())

    // Handle chat messages
    router.Bind("/chat/message", func(ctx *velaros.Context) {
        var msg ChatMessage
        if err := ctx.Unmarshal(&msg); err != nil {
            ctx.Send(ErrorResponse{Error: "invalid message"})
            return
        }

        log.Printf("Received message from %s: %s", msg.Username, msg.Text)

        // Echo back to client
        ctx.Reply(ChatResponse{
            Status: "received",
            Text:   msg.Text,
        })
    })

    // Mount the WebSocket router at /ws
    http.Handle("/ws", router)

    log.Println("WebSocket server listening on ws://localhost:8080/ws")
    http.ListenAndServe(":8080", nil)
}
```

## Core Concepts

### Message Format

Velaros doesn't enforce any specific message format - it's entirely defined by the middleware you choose. Messages are raw bytes until middleware parses them and sets up marshallers/unmarshallers for your handlers.

The framework requires middleware to extract two pieces of information from inbound messages:

- **Message Path** - For routing messages to handlers
- **Message ID** - For request/reply correlation

Middleware does this by calling `ctx.SetMessagePath()` and `ctx.SetMessageID()`, then setting up `ctx.SetMessageUnmarshaler()` and `ctx.SetMessageMarshaller()` for encoding/decoding message data.

Message IDs are required for bidirectional request/reply patterns. When a client sends a message with an ID, handlers can use `Reply()` to send a response with the same ID. When handlers use `Request()` to query the client, the server generates an ID that the client must echo back in their response for proper correlation.

Velaros provides a JSON middleware as a convenient default, but you can create middleware for Protocol Buffers, MessagePack, CBOR, or any other format. You can even use plain text or binary protocols.

### Context

The Context object is passed to every handler and provides access to the current message, socket information, and utility methods for sending responses. It supports both per-message storage (context-level) and per-connection storage (socket-level).

```go
router.Bind("/user/profile", func(ctx *velaros.Context) {
    // Per-message storage (scoped to this message)
    ctx.Set("requestTime", time.Now())

    // Per-connection storage (persists across all messages from this client)
    ctx.SetOnSocket("lastActivity", time.Now())

    // Retrieve values with ok check
    if userID, ok := ctx.GetFromSocket("userID"); ok {
        log.Printf("Request from user: %v", userID)
    }

    // Or use MustGet (panics if key not found)
    userID := ctx.MustGetFromSocket("userID")

    // Send a response
    ctx.Send(ProfileResponse{Name: "Alice", Email: "alice@example.com"})
})
```

### Middleware

Middleware functions execute before handlers in the chain. They can modify the context, perform authentication, log requests, or short-circuit the chain by not calling Next(). Middleware can be applied globally or to specific path patterns.

```go
// Global middleware - runs for all messages
router.Use(func(ctx *velaros.Context) {
    log.Printf("Received message on path: %s", ctx.Path())
    ctx.Next()
    log.Printf("Finished processing message")
})

// Pattern-specific middleware - runs only for matching paths
router.Use("/admin/**", func(ctx *velaros.Context) {
    token, ok := ctx.GetFromSocket("authToken")
    if !ok {
        ctx.Send(ErrorResponse{Error: "unauthorized"})
        return // Don't call Next() - short-circuit the chain
    }
    // Validate token...
    ctx.Next()
})
```

## Built-in Middleware

### JSON Middleware

Automatically handles JSON encoding and decoding of messages. Sets up unmarshalers for parsing incoming message data and marshallers for encoding outbound responses. Includes special handling for error types and field validation errors.

```go
import "github.com/RobertWHurst/velaros/middleware/json"

router.Use(json.Middleware())

type UserData struct {
    Username string `json:"username"`
    Email    string `json:"email"`
}

router.Bind("/user/create", func(ctx *velaros.Context) {
    var user UserData
    if err := ctx.Unmarshal(&user); err != nil {
        ctx.Send(ErrorResponse{Error: "invalid data"})
        return
    }

    // Process user...
    ctx.Reply(UserData{Username: user.Username, Email: user.Email})
})
```

### Set Middleware

Provides multiple variants for setting values:

- **set** - Sets static values on context (per-message scope)
- **setfn** - Sets computed values on context using a function
- **setvalue** - Sets dereferenced pointer values on context
- **socketset** - Sets static values on socket (per-connection scope)
- **socketsetfn** - Sets computed values on socket
- **socketsetvalue** - Sets dereferenced pointer values on socket

```go
import (
    "github.com/RobertWHurst/velaros/middleware/set"
    "github.com/RobertWHurst/velaros/middleware/setfn"
    "github.com/RobertWHurst/velaros/middleware/socketset"
)

// Set static value on each message
router.Use(set.Middleware("apiVersion", "v1"))

// Set computed value on each message
router.Use(setfn.Middleware("requestID", func() string {
    return uuid.NewString()
}))

// Set value on socket (persists across all messages)
router.Use("/auth/login", socketset.Middleware("authenticated", true))
```

## Integration with HTTP Servers

Velaros implements the standard `http.Handler` interface, making it compatible with any Go HTTP router or framework. It can be mounted at any path and will automatically handle WebSocket upgrade requests.

```go
// Standard net/http
router := velaros.NewRouter()
http.Handle("/ws", router)
http.ListenAndServe(":8080", nil)

// Gorilla Mux
mux := mux.NewRouter()
mux.Handle("/ws", router)

// Chi
r := chi.NewRouter()
r.Handle("/ws", router)

// Gin
ginRouter := gin.Default()
ginRouter.Any("/ws", gin.WrapH(router))

// Echo
e := echo.New()
e.Any("/ws", echo.WrapHandler(router))
```

You can also use it alongside your existing HTTP routes:

```go
// Serve both HTTP and WebSocket on the same server
http.HandleFunc("/", handleHome)
http.HandleFunc("/api/users", handleUsers)
http.Handle("/ws", velarosRouter)  // WebSocket endpoint
http.ListenAndServe(":8080", nil)
```

## Routing

Velaros supports pattern-based routing for WebSocket messages. Routes can match exact paths, include named parameters, or use wildcards. Parameters are extracted and made available through the context. Middleware can also be scoped to specific path patterns.

```go
// Exact path match
router.Bind("/users/list", func(ctx *velaros.Context) {
    ctx.Send(UserListResponse{Users: getAllUsers()})
})

// Named parameters
router.Bind("/users/:id", func(ctx *velaros.Context) {
    userID := ctx.Params().Get("id")
    user := getUserByID(userID)
    ctx.Send(UserResponse{User: user})
})

// Wildcard matching
router.Bind("/files/*", func(ctx *velaros.Context) {
    path := ctx.Path()
    log.Printf("File request: %s", path)
})
```

### Message Path Patterns

Velaros supports fairly powerful message path patterns. The following is a list of supported pattern chunk types.

- Static - `/a/b/c` - Matches the exact path
- Wildcard - `/a/*/c` - Pattern segments with a single `*` match any path segment
- Dynamic - `/a/:b/c` - Pattern segments prefixed with `:` match any path segment and the value of this segment from the matched path is available via the `Params` method, and will be filled under a key matching the name of the pattern segment, ie: pattern of `/a/:b/c` will match `/a/1/c` and the value of `b` in the params will be `1`

Pattern chunks can also be suffixed with additional modifiers.

- `?` - Optional - `/a/:b?/c` - Matches `/a/c` and `/a/1/c`
- `*` - Greedy - `/a/:b*/c` - Matches `/a/c` and `/a/1/2/3/c`
- `+` - One or more - `/a/:b+/c` - Matches `/a/1/c` and `/a/1/2/3/c` but not `/a/c`

You can also provide a regular expression to restrict matches for a pattern chunk.

- `/a/:b(\\d+)/c` - Matches `/a/1/c` and `/a/2/c` but not `/a/b/c`

You can escape any of the special characters used by these operators by prefixing them with a `\\`.

- `/a/\\:b/c` - Matches `/a/:b/c`

And all of these can be combined.

- `/a/:b(\\d+)/*?/(d|e)+` - Matches `/a/1/d`, `/a/1/e`, `/a/2/c/d/e/f/g`, and `/a/3/1/d` but not `/a/b/c`, `/a/1`, or `/a/1/c/f`

This is all most likely overkill, but if you ever need it, it's here.

### Handler and Middleware Ordering

Handlers and middleware are executed in the order they are added to the router. This means that a handler added before another will always be checked for a match against the incoming message first regardless of the path pattern. This means you can easily predict how your handlers will be executed.

It also means that your handlers with more specific patterns should be added before any others that may share a common match.

```go
router.Bind("/album/:id(\\d{24})", GetAlbumByID)
router.Bind("/album/:name", GetAlbumsByName)
```

## Bidirectional Communication

Unlike HTTP, WebSocket connections are bidirectional - the server can send messages to clients at any time, not just in response to requests. Velaros provides several communication patterns to leverage this capability.

### Send and Reply

Use `Send()` to send a message without expecting a response, or `Reply()` to respond to a message that includes an ID:

```go
router.Bind("/notify", func(ctx *velaros.Context) {
    // Reply to the original message (preserves message ID)
    ctx.Reply(AckResponse{Status: "received"})

    // Later, send additional messages
    time.Sleep(time.Second)
    ctx.Send(NotificationMessage{Text: "Processing complete"})
})
```

### Request and Response

The server can initiate requests to clients and wait for responses using the `Request()` family of methods:

```go
type ConfirmRequest struct {
    Action string `json:"action"`
}

type ConfirmResponse struct {
    Confirmed bool `json:"confirmed"`
}

router.Bind("/delete/:id", func(ctx *velaros.Context) {
    id := ctx.Params().Get("id")

    // Ask the client for confirmation
    response, err := ctx.Request(ConfirmRequest{
        Action: "delete item " + id,
    })
    if err != nil {
        ctx.Send(ErrorResponse{Error: "confirmation timeout"})
        return
    }

    // Parse the response
    var confirm ConfirmResponse
    json.Unmarshal(response.([]byte), &confirm)

    if confirm.Confirmed {
        deleteItem(id)
        ctx.Send(SuccessResponse{Status: "deleted"})
    } else {
        ctx.Send(ErrorResponse{Error: "cancelled"})
    }
})
```

### Typed Requests with RequestInto

For cleaner code, use `RequestInto()` which automatically unmarshals the response:

```go
router.Bind("/delete/:id", func(ctx *velaros.Context) {
    id := ctx.Params().Get("id")

    var confirm ConfirmResponse
    if err := ctx.RequestInto(ConfirmRequest{
        Action: "delete item " + id,
    }, &confirm); err != nil {
        ctx.Send(ErrorResponse{Error: "confirmation timeout"})
        return
    }

    if confirm.Confirmed {
        deleteItem(id)
        ctx.Send(SuccessResponse{Status: "deleted"})
    }
})
```

### Request Timeouts and Cancellation

Control request timeouts and cancellation using context-aware variants:

```go
// Custom timeout
var response ConfirmResponse
err := ctx.RequestIntoWithTimeout(
    ConfirmRequest{Action: "approve"},
    &response,
    30 * time.Second,
)

// Full context control
cancelCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

err := ctx.RequestIntoWithContext(
    cancelCtx,
    ConfirmRequest{Action: "approve"},
    &response,
)
```

## Advanced Usage

### Authentication

Authentication state can be stored at the socket level so it persists across all messages from that connection. Use helper functions to check authentication state in your handlers.

```go
type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type LoginResponse struct {
    Token string `json:"token"`
}

type UserProfile struct {
    Email string `json:"email"`
    Name  string `json:"name"`
}

// Helper function to check if socket is authenticated
func isAuthenticated(ctx *velaros.Context) bool {
    _, ok := ctx.GetFromSocket("authToken")
    return ok
}

// Helper function to get authenticated username
func getUsername(ctx *velaros.Context) (string, bool) {
    username, ok := ctx.GetFromSocket("username")
    if !ok {
        return "", false
    }
    return username.(string), true
}

// Login handler - stores auth info on socket
router.Bind("/auth/login", func(ctx *velaros.Context) {
    var req LoginRequest
    if err := ctx.Unmarshal(&req); err != nil {
        ctx.Send(ErrorResponse{Error: "invalid request"})
        return
    }

    if validateCredentials(req.Username, req.Password) {
        token := generateToken(req.Username)

        // Store authentication on socket (persists for connection lifetime)
        ctx.SetOnSocket("authToken", token)
        ctx.SetOnSocket("username", req.Username)

        ctx.Reply(LoginResponse{Token: token})
    } else {
        ctx.Send(ErrorResponse{Error: "invalid credentials"})
    }
})

// Protected handler - uses helper to check auth
router.Bind("/user/profile", func(ctx *velaros.Context) {
    if !isAuthenticated(ctx) {
        ctx.Send(ErrorResponse{Error: "unauthorized"})
        return
    }

    username, _ := getUsername(ctx)
    profile := getUserProfile(username)
    ctx.Reply(UserProfile{Email: profile.Email, Name: profile.Name})
})
```

### Error Handling

Errors can be captured using error-handling middleware that runs after other handlers. Check the context's Error field and send appropriate error responses. Errors set during handler execution are automatically captured with stack traces.

```go
// Error handling middleware - runs after handlers
router.Use(func(ctx *velaros.Context) {
    ctx.Next()

    if ctx.Error != nil {
        log.Printf("Error: %v", ctx.Error)
        ctx.Send(ErrorResponse{Error: ctx.Error.Error()})
    }
})

// Handlers can set errors
router.Bind("/data/process", func(ctx *velaros.Context) {
    var data ProcessData
    if err := ctx.Unmarshal(&data); err != nil {
        ctx.Error = fmt.Errorf("invalid data: %w", err)
        return
    }

    if err := processData(data); err != nil {
        ctx.Error = fmt.Errorf("processing failed: %w", err)
        return
    }

    ctx.Send(SuccessResponse{Status: "processed"})
})
```

## Message Types

Velaros automatically detects the message type (text or binary) for each incoming WebSocket message and uses the same type for all responses to that message. This ensures compatibility with clients that expect consistent message types.

```go
// Client sends text message
// -> Server automatically uses text for the response

// Client sends binary message
// -> Server automatically uses binary for the response

// The message type is detected per-message and stored in the context
router.Bind("/echo", func(ctx *velaros.Context) {
    // Response will use the same message type (text or binary)
    // that the client used for this message
    ctx.Reply(EchoResponse{Message: "echo"})
})
```

## Performance

Context pooling reduces memory allocations by reusing context objects across requests. Raw message bytes are passed directly to handlers without unnecessary copying. Each message is handled concurrently in its own goroutine for maximum throughput.

**Key optimizations:**

- Context objects are pooled and reused via `sync.Pool`
- Zero-copy message passing where possible
- Concurrent message processing (each message in its own goroutine)
- Efficient pattern matching for routing
- Minimal allocations in hot paths

## Architecture

Velaros is designed for real-time, persistent WebSocket connections where each connection maintains state across multiple messages. Handlers can send messages at any time, not just in response to incoming messages. For cross-connection communication like broadcasts, integrate with external message queues or event streams.

```go
// Handlers can send messages at any time, not just in responses
router.Bind("/subscribe/updates", func(ctx *velaros.Context) {
    // Acknowledge subscription
    ctx.Reply(SuccessResponse{Status: "subscribed"})

    // Later, when an event occurs (triggered by external system):
    // - Use a message queue (Redis, NATS, Kafka, etc.)
    // - Subscribe to events in your handler
    // - Send updates to client when events arrive

    // Example with a channel (for single-server setups):
    userID, _ := ctx.GetFromSocket("userID")
    updatesChan := subscribeToUpdates(userID)
    go func() {
        for update := range updatesChan {
            ctx.Send(UpdateMessage{Data: update})
        }
    }()
})
```

## Examples

### Basic Echo Server

```go
package main

import (
    "log"
    "net/http"

    "github.com/RobertWHurst/velaros"
    "github.com/RobertWHurst/velaros/middleware/json"
)

type EchoRequest struct {
    Message string `json:"message"`
}

type EchoResponse struct {
    Echo string `json:"echo"`
}

func main() {
    router := velaros.NewRouter()
    router.Use(json.Middleware())

    router.Bind("/echo", func(ctx *velaros.Context) {
        var req EchoRequest
        if err := ctx.Unmarshal(&req); err != nil {
            return
        }
        ctx.Reply(EchoResponse{Echo: req.Message})
    })

    log.Println("Echo server listening on :8080")
    http.ListenAndServe(":8080", router)
}
```

### Chat Room

```go
package main

import (
    "log"
    "net/http"
    "sync"

    "github.com/RobertWHurst/velaros"
    "github.com/RobertWHurst/velaros/middleware/json"
)

type JoinRequest struct {
    Username string `json:"username"`
}

type ChatMessage struct {
    Username string `json:"username"`
    Text     string `json:"text"`
}

type Broadcast struct {
    From string `json:"from"`
    Text string `json:"text"`
}

var (
    clients   = make(map[string]*velaros.Context)
    clientsMx sync.Mutex
)

func broadcast(msg Broadcast) {
    clientsMx.Lock()
    defer clientsMx.Unlock()

    for _, client := range clients {
        client.Send(msg)
    }
}

func main() {
    router := velaros.NewRouter()
    router.Use(json.Middleware())

    router.Bind("/join", func(ctx *velaros.Context) {
        var req JoinRequest
        if err := ctx.Unmarshal(&req); err != nil {
            return
        }

        ctx.SetOnSocket("username", req.Username)

        clientsMx.Lock()
        clients[ctx.SocketID()] = ctx
        clientsMx.Unlock()

        log.Printf("%s joined", req.Username)
    })

    router.Bind("/message", func(ctx *velaros.Context) {
        var msg ChatMessage
        if err := ctx.Unmarshal(&msg); err != nil {
            return
        }

        username := ctx.MustGetFromSocket("username").(string)
        broadcast(Broadcast{
            From: username,
            Text: msg.Text,
        })
    })

    log.Println("Chat server listening on :8080")
    http.ListenAndServe(":8080", router)
}
```

### Request/Reply Pattern

```go
package main

import (
    "log"
    "net/http"
    "time"

    "github.com/RobertWHurst/velaros"
    "github.com/RobertWHurst/velaros/middleware/json"
)

type PingRequest struct {
    Timestamp int64 `json:"timestamp"`
}

type PongResponse struct {
    Timestamp int64 `json:"timestamp"`
    Latency   int64 `json:"latency"`
}

func main() {
    router := velaros.NewRouter()
    router.Use(json.Middleware())

    router.Bind("/ping", func(ctx *velaros.Context) {
        var req PingRequest
        if err := ctx.Unmarshal(&req); err != nil {
            return
        }

        now := time.Now().UnixMilli()
        latency := now - req.Timestamp

        ctx.Reply(PongResponse{
            Timestamp: now,
            Latency:   latency,
        })
    })

    log.Println("Ping server listening on :8080")
    http.ListenAndServe(":8080", router)
}
```

## Contributing

Pull requests are welcome!

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [Navaros](https://github.com/RobertWHurst/Navaros) - HTTP framework for Go (companion project, shares similar API design)
