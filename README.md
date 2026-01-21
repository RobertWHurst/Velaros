# Velaros

A lightweight, flexible WebSocket framework for Go that brings HTTP-style message routing to WebSocket connections. Build real-time applications with powerful pattern-based routing, bidirectional request/reply communication, and composable middleware.

Unlike traditional WebSocket libraries that give you a raw connection, Velaros routes messages to handlers based on message paths - similar to HTTP routing but over persistent WebSocket connections. This enables clean, maintainable real-time applications with familiar patterns.

Velaros implements the standard `http.Handler` interface, so it works seamlessly with any Go HTTP router or framework - just mount it on a path like `/ws` and it handles the WebSocket upgrade automatically.

It also integrates as middleware with Navaros, its sister HTTP framework, through `router.Middleware()`. Learn more about Navaros at [github.com/RobertWHurst/Navaros](https://github.com/RobertWHurst/Navaros).

[![Go Reference](https://pkg.go.dev/badge/github.com/RobertWHurst/velaros.svg)](https://pkg.go.dev/github.com/RobertWHurst/velaros)
[![Go Report Card](https://goreportcard.com/badge/github.com/RobertWHurst/velaros)](https://goreportcard.com/report/github.com/RobertWHurst/velaros)
[![CI](https://github.com/RobertWHurst/Velaros/actions/workflows/ci.yml/badge.svg)](https://github.com/RobertWHurst/Velaros/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/RobertWHurst/d9c059efcefa498e9047b1f7579da3c9/raw/velaros-coverage.json)](https://github.com/RobertWHurst/Velaros/actions/workflows/ci.yml)
[![GitHub release](https://img.shields.io/github/v/release/RobertWHurst/velaros)](https://github.com/RobertWHurst/velaros/releases)
[![License](https://img.shields.io/github/license/RobertWHurst/velaros)](LICENSE)
[![Sponsor](https://img.shields.io/static/v1?label=Sponsor&message=%E2%9D%A4&logo=GitHub&color=%23fe8e86)](https://github.com/sponsors/RobertWHurst)

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
  - [Message Format](#message-format)
  - [Context](#context)
  - [Context Lifecycle](#context-lifecycle)
  - [Middleware](#middleware)
- [Built-in Middleware](#built-in-middleware)
  - [JSON Middleware](#json-middleware)
  - [MessagePack Middleware](#messagepack-middleware)
  - [Protocol Buffers Middleware](#protocol-buffers-middleware)
  - [Set Middleware Variants](#set-middleware-variants)
- [Integration with HTTP Servers](#integration-with-http-servers)
- [Configuration](#configuration)
- [Routing](#routing)
  - [Route Introspection](#route-introspection)
- [Connection Lifecycle](#connection-lifecycle)
- [Bidirectional Communication](#bidirectional-communication)
- [Advanced Usage](#advanced-usage)
  - [Authentication](#authentication)
  - [Error Handling](#error-handling)
  - [API Gateway Integration](#api-gateway-integration)
- [Message Types](#message-types)
- [Performance](#performance)
- [Architecture](#architecture)
- [Use with Navaros](#use-with-navaros)
- [Testing](#testing)
- [Help Welcome](#help-welcome)
- [License](#license)
- [Related Projects](#related-projects)

## Features

- 🚀 **High Performance** - Context pooling and efficient message routing
- 🔄 **Bidirectional** - Full duplex communication where both client and server can initiate messages and await responses
- 🎯 **Powerful Patterns** - Flexible routing with parameters, wildcards, regex constraints, and modifiers
- 🔌 **Middleware** - Composable middleware for authentication, logging, and more
- 🔁 **Lifecycle Hooks** - UseOpen and UseClose handlers for connection initialization and cleanup
- 📦 **Type Detection** - Automatic text/binary message type handling
- ⏱️ **Timeout Control** - Request timeouts and context cancellation for server→client requests
- 🧩 **Extensible** - Simple interfaces for custom handlers and middleware

## Installation

```bash
go get github.com/RobertWHurst/velaros
```

## Quick Start

Here's a simple game server that demonstrates the core concepts: HTTP-like message routing, socket state persistence, and bidirectional communication where the server can request data from clients.

```go
router := velaros.NewRouter()
router.Use(json.Middleware())

// Player joins game
router.Bind("/game/join", func(ctx *velaros.Context) {
    var player JoinRequest
    ctx.ReceiveInto(&player)

    ctx.SetOnSocket("playerName", player.PlayerName)
    ctx.Send(JoinResponse{PlayerID: "p123", Status: "joined"})
})

// Player performs action
router.Bind("/game/action", func(ctx *velaros.Context) {
    var action PlayerAction
    ctx.ReceiveInto(&action)

    playerName := ctx.MustGetFromSocket("playerName").(string)
    log.Printf("%s performed action: %s", playerName, action.Type)

    ctx.Send(ActionResponse{Success: true})
})

// Server periodically syncs client state
router.Bind("/game/sync", func(ctx *velaros.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            var state ClientState
            ctx.RequestInto(SyncRequest{ServerTime: time.Now().Unix()}, &state)
            log.Printf("Player at: %.2f, %.2f", state.Position.X, state.Position.Y)
        case <-ctx.Done():
            return
        }
    }
})

http.Handle("/ws", router)
http.ListenAndServe(":8080", nil)
```

Here's what a client might look like written in JavaScript.

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

// Join game
ws.send(JSON.stringify({
    path: '/game/join',
    data: { playerName: 'Alice' }
}));

// Perform action
ws.send(JSON.stringify({
    path: '/game/action',
    data: { type: 'jump' }
}));

// Handle server-initiated sync requests
ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    
    if (msg.path === '/game/sync') {
        // Server requesting client state - reply with same ID
        ws.send(JSON.stringify({
            id: msg.id,
            data: { position: { x: 10, y: 20 }, health: 100 }
        }));
    }
};
```

**Note:** This example uses the JSON middleware. It expects messages to be structured with `{path, id, data}`. See [Built-in Middleware](#built-in-middleware) for MessagePack and Protocol Buffers formats or to learn more about the JSON middleware.

## Core Concepts

### Message Format

Velaros doesn't enforce any specific message format - it's entirely defined by the encoding/decoding middleware you choose. Messages are raw bytes until middleware parses them.

The framework requires encoding/decoding middleware to extract two pieces of information from inbound messages:

- **Message Path** - For routing messages to handlers
- **Message ID** - For request/reply correlation

Middleware does this by calling `ctx.SetMessagePath()` and `ctx.SetMessageID()`.

Encoding/decoding middleware can also provide marshallers and unmarshallers by calling `ctx.SetMessageMarshaller()` and `ctx.SetMessageUnmarshaler()`. This allows handlers to unmarshal incoming message data into structs and pass structs to the context's sending methods for automatic encoding.

Message IDs enable multi-message conversations within a single handler. When a client sends a message with an ID, that same ID is used for all communication with that handler instance. The handler can call `Send()` to respond and `Receive()` to get subsequent messages from the client - all messages using the same ID are automatically routed to that handler.

**Message Metadata** - Messages can optionally include metadata alongside the message data. Middleware can extract metadata by calling `ctx.SetMessageMeta()`, making it available through `ctx.Meta(key)`. This is useful for passing contextual information like authentication tokens, tracing IDs, request IDs, or other cross-cutting concerns that don't belong in the message data itself.

```go
router.Bind("/api/data", func(ctx *velaros.Context) {
    if userId, ok := ctx.Meta("userId"); ok {
        log.Printf("Request from user: %v", userId)
    }

    if traceId, ok := ctx.Meta("traceId"); ok {
        log.Printf("Trace ID: %v", traceId)
    }

    ctx.Send(DataResponse{Items: getData()})
})
```

Velaros provides encoding/decoding middleware for common formats: **JSON** (human-readable), **MessagePack** (binary, high-performance), and **Protocol Buffers** (strongly-typed, cross-language, production-grade). You can also create custom encoding/decoding middleware for other formats like CBOR, or even plain text/binary protocols. See the [Built-in Middleware](#built-in-middleware) section for details on each format.

### Context

The Context object is passed to every handler and provides access to the current message, socket information, and utility methods for sending responses. It's your primary interface for interacting with WebSocket connections - reading message data, extracting route parameters, storing state, sending responses, and closing connections.

Context supports two types of storage: per-message storage that's scoped to a single message and cleared after the handler completes, and per-connection storage that persists for the entire WebSocket connection lifetime. This dual storage model lets you handle both transient request data and persistent connection state efficiently.

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

**When to use Message vs Socket Storage:**

**Message Storage (`ctx.Set()`, `ctx.Get()`):**

- Request-specific data (request ID, start time, parsed request data)
- Data that should NOT persist across messages
- Temporary values for passing between middleware
- Automatically cleared after message processing completes

**Socket Storage (`ctx.SetOnSocket()`, `ctx.GetFromSocket()`):**

- Connection-level data (user ID, session ID, auth tokens)
- Data that persists for the entire connection lifetime
- Authentication state, user preferences, client metadata
- Thread-safe - safely accessed concurrently from multiple message handlers
- Cleared when the connection closes

**Examples:**

```go
// Authentication middleware stores user on socket
router.Use("/api", func(ctx *velaros.Context) {
    token := getTokenFromMessage(ctx)
    if user, err := validateToken(token); err == nil {
        ctx.SetOnSocket("user", user)          // Persists across messages
        ctx.SetOnSocket("authenticated", true)  // Persists across messages
    }
    ctx.Next()
})

// Handler uses both types of storage
router.Bind("/api/data/process", func(ctx *velaros.Context) {
    // Message storage - per-request
    ctx.Set("requestID", uuid.New())      // Only for this message
    ctx.Set("startTime", time.Now())     // Only for this message

    // Socket storage - connection-wide
    user, _ := ctx.GetFromSocket("user") // Persists from auth middleware

    processData(user)
    ctx.Send(SuccessResponse{})
})

// Logging middleware reads both
router.Use(func(ctx *velaros.Context) {
    ctx.Next()

    // Message-specific
    startTime, _ := ctx.Get("startTime")
    duration := time.Since(startTime.(time.Time))

    // Connection-specific
    user, _ := ctx.GetFromSocket("user")

    log.Printf("User %v processed message in %v", user, duration)
})
```

**Accessing Request Headers:**

The Context provides access to HTTP headers from the initial WebSocket handshake request. This is useful for authentication:

```go
router.UseOpen(func(ctx *velaros.Context) {
    // Authenticate using Authorization header
    if authHeader := ctx.Headers().Get("Authorization"); authHeader != "" {
        token := strings.TrimPrefix(authHeader, "Bearer ")
        if user, err := validateJWT(token); err == nil {
            ctx.SetOnSocket("user", user)
            ctx.SetOnSocket("authenticated", true)
        }
    }
})

// Protected handler checks authentication
router.Bind("/admin/users", func(ctx *velaros.Context) {
    if authenticated, _ := ctx.GetFromSocket("authenticated"); authenticated != true {
        ctx.Send(ErrorResponse{Error: "unauthorized"})
        return
    }
    
    ctx.Send(AdminUsersResponse{Users: getUsers()})
})
```

**Note:** Headers are from the initial WebSocket handshake when the WebSocket connection was established. All communication in handlers uses WebSocket communication, not HTTP.

### Context Lifecycle

**Important:** Context objects are pooled and reused for performance. When a handler returns, its context is immediately returned to the pool and may be reused for a different message. This means **handlers must block until all operations using the context are complete**.

If you spawn a goroutine or set up a callback that references the context after the handler returns, those operations will fail with an error: `"context cannot be used after handler returns - handlers must block until all operations complete"`.

**Wrong - Don't do this:**

```go
// ❌ This will fail - goroutine uses context after handler returns
router.Bind("/subscribe", func(ctx *velaros.Context) {
    ctx.Send(SubscribeResponse{Status: "subscribed"})

    go func() {
        time.Sleep(time.Second)
        ctx.Send(Update{Data: "..."}) // ERROR: context already returned to pool
    }()
    // Handler returns immediately - context is freed
})
```

**Right - Handler blocks:**

```go
// ✓ Correct - handler blocks until all operations complete
router.Bind("/subscribe", func(ctx *velaros.Context) {
    ctx.Send(SubscribeResponse{Status: "subscribed"})

    subscription := messageQueue.Subscribe("updates")
    defer subscription.Unsubscribe()

    // Handler blocks here, keeping context alive
    for {
        select {
        case msg := <-subscription.Messages():
            ctx.Send(Update{Data: msg}) // Safe: handler still owns context
        case <-ctx.Done():
            return // Connection closed, clean exit
        }
    }
})
```

Handlers only need to block as long as they need to use the context. For quick request/reply handlers, this means they return immediately after sending the response. For subscription-based handlers that send multiple messages over time, they block for as long as the subscription is active. The key rule: don't return from the handler while operations that use the context are still pending.

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
router.Use("/admin", func(ctx *velaros.Context) {
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

**Message Structure:**

The JSON middleware expects messages in this format:

```json
{
    "path": "/user/create",
    "id": "optional-request-id",
    "data": {
        "username": "alice",
        "email": "alice@example.com"
    },
    "meta": {
        "userId": "user-123",
        "traceId": "trace-456"
    }
}
```

- **path** (string, required): Routes the message to the appropriate handler
- **id** (string, optional): For request/reply correlation - include when you expect a reply
- **data** (any, optional): The actual message payload that gets passed to `ctx.ReceiveInto()`
- **meta** (object, optional): Metadata for passing contextual information like auth tokens or tracing IDs

Responses follow the same structure:

```json
{
    "id": "optional-request-id",
    "data": {
        "username": "alice",
        "email": "alice@example.com"
    }
}
```

**Server-Side Usage:**

```go
import "github.com/RobertWHurst/velaros/middleware/json"

router.Use(json.Middleware())

type UserData struct {
    Username string `json:"username"`
    Email    string `json:"email"`
}

router.Bind("/user/create", func(ctx *velaros.Context) {
    var user UserData
    // ReceiveInto extracts and unmarshals the "data" field from the message
    if err := ctx.ReceiveInto(&user); err != nil {
        ctx.Send(ErrorResponse{Error: "invalid data"})
        return
    }

    // Process user...
    // Send responds with: {"id": "...", "data": {"username": "...", "email": "..."}}
    ctx.Send(UserData{Username: user.Username, Email: user.Email})
})
```

**Note:** This message structure is specific to the JSON middleware. Velaros itself is format-agnostic - you can create custom middleware for Protocol Buffers, MessagePack, or any other format with different message structures.

### MessagePack Middleware

MessagePack is a binary serialization format that's significantly faster and more compact than JSON. It's ideal for high-throughput systems, mobile applications, and scenarios where bandwidth or performance is critical.

**Why use MessagePack:**

- **5x faster** than JSON for serialization/deserialization
- **Smaller message sizes** - reduces bandwidth usage
- **Binary efficiency** - better for mobile and IoT devices
- **Drop-in replacement** - same API as JSON middleware

**Message Structure:**

MessagePack uses the same envelope structure as JSON (`{id, path, data}`), but encoded in binary format instead of text:

```go
// MessagePack binary encoding of: {id: "msg-123", path: "/users/get", data: {...}}
```

**Server-Side Usage:**

```go
import "github.com/RobertWHurst/velaros/middleware/msgpack"

router := velaros.NewRouter()
router.Use(msgpack.Middleware())

type UserRequest struct {
    UserID int64  `msgpack:"user_id"`
    Name   string `msgpack:"name"`
}

router.Bind("/users/create", func(ctx *velaros.Context) {
    var req UserRequest
    if err := ctx.ReceiveInto(&req); err != nil {
        ctx.Send(ErrorResponse{Error: "invalid data"})
        return
    }

    // Process user...
    ctx.Send(UserResponse{UserID: req.UserID, Name: req.Name})
})
```

**Client-Side Usage (JavaScript):**

Install the MessagePack library: `npm install @msgpack/msgpack`

```javascript
import { encode, decode } from '@msgpack/msgpack';

const ws = new WebSocket('ws://localhost:8080/ws', 'velaros-msgpack');

ws.binaryType = 'arraybuffer';  // Important: use arraybuffer for binary data

ws.onopen = () => {
    // Encode message to MessagePack binary
    const message = {
        path: '/users/create',
        id: 'msg-123',
        user_id: 42,
        name: 'Alice'
    };
    const encoded = encode(message);
    ws.send(encoded);
};

ws.onmessage = (event) => {
    // Decode MessagePack binary to object
    const message = decode(new Uint8Array(event.data));
    console.log('Received:', message);
};
```

**When to Use:**

- Real-time data streaming systems
- Mobile apps with limited bandwidth
- IoT devices with constrained resources
- Game servers with many concurrent connections
- Any scenario where JSON performance is a bottleneck

### Protocol Buffers Middleware

Protocol Buffers (protobuf) is Google's language-neutral, platform-neutral serialization format. It provides strong typing, schema validation, and excellent performance with compact binary encoding. Protobuf is the gold standard for cross-language communication in distributed systems.

**Why use Protocol Buffers:**

- **Type safety** - Strongly-typed schemas with compile-time validation prevent runtime errors
- **Language-agnostic** - Same .proto files generate code for Go, Java, Python, C++, JavaScript, and more
- **Highly efficient** - Compact binary format with minimal overhead, faster than JSON and comparable to MessagePack
- **Schema evolution** - Add, rename, or deprecate fields without breaking existing clients through field numbering
- **Backward/forward compatibility** - Older clients work with newer servers and vice versa
- **Code generation** - Automatically generates serialization code, getters, setters, and builders
- **gRPC compatibility** - De facto standard for gRPC and modern microservices architectures
- **Industry standard** - Used by Google, Netflix, Square, and thousands of companies for production systems

**Key Feature:** Work with standard `.proto` files - no Velaros-specific modifications needed. The middleware handles envelope wrapping transparently.

**Complete Workflow:**

**Step 1: Define your `.proto` schema:**

```protobuf
// user.proto
syntax = "proto3";
package myapp;
option go_package = "github.com/myuser/myapp/proto";

message CreateUserRequest {
  string username = 1;
  string email = 2;
}

message UserResponse {
  int64 user_id = 1;
  string username = 2;
  string email = 3;
}
```

**Step 2: Generate Go code:**

```bash
protoc --go_out=. --go_opt=paths=source_relative user.proto
```

**Step 3: Use with Velaros (no envelope awareness needed):**

```go
import (
    "github.com/RobertWHurst/velaros/middleware/protobuf"
    pb "github.com/myuser/myapp/proto"
)

router := velaros.NewRouter()
router.Use(protobuf.Middleware())

router.Bind("/users/create", func(ctx *velaros.Context) {
    var req pb.CreateUserRequest
    if err := ctx.ReceiveInto(&req); err != nil {
        ctx.Send(&pb.ErrorResponse{Error: "invalid data"})
        return
    }

    // Process user...
    ctx.Send(&pb.UserResponse{
        UserId:   123,
        Username: req.Username,
        Email:    req.Email,
    })
})
```

**How It Works:**

The middleware uses an internal envelope to add routing metadata (`id`, `path`) to your protobuf messages. This envelope is completely transparent - on the server side, you work directly with your generated protobuf types. The middleware handles wrapping and unwrapping automatically.

**Client-Side Usage (JavaScript):**

Clients need to wrap messages in the Velaros envelope. The envelope schema is available in the [protobuf package](https://github.com/RobertWHurst/Velaros/blob/master/middleware/protobuf/envelope.proto).

```javascript
import protobuf from 'protobufjs';

// Load your .proto definitions
const root = await protobuf.load('user.proto');
const CreateUserRequest = root.lookupType('myapp.CreateUserRequest');
const UserResponse = root.lookupType('myapp.UserResponse');

// Load Velaros envelope from the protobuf middleware package
const Envelope = root.lookupType('velaros.protobuf.Envelope');

const ws = new WebSocket('ws://localhost:8080/ws', 'velaros-protobuf');
ws.binaryType = 'arraybuffer';

ws.onopen = () => {
    // Create your message
    const req = CreateUserRequest.create({
        username: 'alice',
        email: 'alice@example.com'
    });

    // Wrap in envelope
    const envelope = Envelope.create({
        id: 'msg-123',
        path: '/users/create',
        data: CreateUserRequest.encode(req).finish()
    });

    // Send encoded envelope
    ws.send(Envelope.encode(envelope).finish());
};

ws.onmessage = (event) => {
    // Decode envelope
    const envelope = Envelope.decode(new Uint8Array(event.data));

    // Decode your message from envelope.data
    const response = UserResponse.decode(envelope.data);
    console.log('User created:', response.userId);
};
```

**When to Use:**

- Microservices architectures
- Polyglot systems (multiple languages)
- Type-safe APIs with schema validation
- Systems requiring backward/forward compatibility
- Integration with gRPC services

**Learn More:** [Official Protocol Buffers tutorial](https://protobuf.dev/getting-started/gotutorial/)

### Set Middleware Variants

The Set middleware family lets you store values on the context or socket as middleware. This is useful for setting up common values that multiple handlers need. Each variant takes a key and value/function as parameters.

**Message-level middleware** (set, setfn, setvalue) stores values scoped to each individual message - values are cleared after the handler completes:

- **set** stores static values on every message - takes a key and value
- **setfn** calls a function on every message and stores the result - takes a key and function. Useful for values that change per message like request IDs or timestamps
- **setvalue** dereferences a pointer and stores the value - takes a key and pointer. Useful when the value might change between messages but you want to capture the current value

**Socket-level middleware** (socketset, socketsetfn, socketsetvalue) stores values scoped to the entire connection - values persist across all messages until the connection closes:

- **socketset** stores static values on the connection - takes a key and value
- **socketsetfn** calls a function once per message and stores the result on the socket - takes a key and function
- **socketsetvalue** dereferences a pointer and stores the value on the socket - takes a key and pointer

```go
import (
    "github.com/RobertWHurst/velaros/middleware/set"
    "github.com/RobertWHurst/velaros/middleware/setfn"
    "github.com/RobertWHurst/velaros/middleware/setvalue"
    "github.com/RobertWHurst/velaros/middleware/socketset"
    "github.com/RobertWHurst/velaros/middleware/socketsetfn"
    "github.com/RobertWHurst/velaros/middleware/socketsetvalue"
)

// Message-level values
router.Use(set.Middleware("version", "1.0.0"))

router.Use(setfn.Middleware("requestID", func() string {
    return uuid.New().String()
}))

maxItems := 100
router.Use(setvalue.Middleware("maxItems", &maxItems))

// Later, maxItems can be changed and setvalue captures the current value per message
maxItems = 200

// Socket-level values
router.UseOpen(socketset.Middleware("region", "us-east"))

router.UseOpen(socketsetfn.Middleware("sessionID", func() string {
    return uuid.New().String()
}))

connectionCount := 0
router.UseOpen(socketsetvalue.Middleware("connectionCount", &connectionCount))

router.Bind("/info", func(ctx *velaros.Context) {
    version := ctx.Get("version").(string)
    requestID := ctx.Get("requestID").(string)
    maxItems := ctx.Get("maxItems").(int) // Gets the dereferenced int value
    
    region := ctx.GetFromSocket("region").(string)
    sessionID := ctx.GetFromSocket("sessionID").(string)
    connCount := ctx.GetFromSocket("connectionCount").(int)
    
    ctx.Send(InfoResponse{
        Version:         version,
        RequestID:       requestID,
        MaxItems:        maxItems,
        Region:          region,
        SessionID:       sessionID,
        ConnectionCount: connCount,
    })
})
```

## Integration with HTTP Servers

Velaros implements the standard `http.Handler` interface, making it compatible with any Go HTTP router or framework. It can be mounted at any path and will automatically handle WebSocket upgrade requests.

```go
// Standard net/http
router := velaros.NewRouter()
http.Handle("/ws", router)
http.ListenAndServe(":8080", nil)

// Navaros
httpRouter := navaros.NewRouter()
httpRouter.Use(router.Middleware())

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

### WebSocket Upgrade

When a client connects to the mounted path, Velaros automatically handles the WebSocket upgrade handshake. If the request is not a WebSocket upgrade request, the router returns a 400 Bad Request error.

**Note:** When using `router.Middleware()` with Navaros, non-WebSocket requests are passed to the next handler in the chain instead of returning an error. This allows you to use Velaros as pathless middleware that automatically upgrades WebSocket requests while allowing HTTP requests to pass through to other handlers.

## Configuration

### Origin Validation

By default, Velaros accepts WebSocket connections from any origin (`*`). For security in production, restrict origins to prevent CSRF attacks:

```go
router := velaros.NewRouter()

// Allow specific origins
router.SetOrigins([]string{
    "https://myapp.com",
    "https://*.myapp.com",  // Wildcards supported
})

http.Handle("/ws", router)
```

This configures the WebSocket handshake to only accept connections from the specified origins. Connections from other origins will be rejected during the upgrade.

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
router.Bind("/events/**", func(ctx *velaros.Context) {
    eventPath := ctx.Path() // e.g., "/events/user/login" or "/events/system/alert"
    log.Printf("Subscribed to: %s", eventPath)
    
    // Stream events to client...
    ctx.Send(SubscriptionResponse{Status: "subscribed", Path: eventPath})
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

### Accessing Route Parameters

Parameters extracted from route patterns are available via `ctx.Params()`, which returns a `MessageParams` type. Parameter names are case-insensitive when retrieved.

**Example:**

```go
router.Bind("/users/:userID/posts/:postID", func(ctx *velaros.Context) {
    userID := ctx.Params().Get("userID")    // Gets "123"
    postID := ctx.Params().Get("postid")    // Case-insensitive: also works

    // Parameters are strings - convert as needed
    userIDInt, _ := strconv.Atoi(userID)

    posts := getUserPosts(userIDInt, postID)
    ctx.Send(posts)
})

// Client sends: {path: "/users/123/posts/456", ...}
```

**Key Points:**

- The `Get()` method does case-insensitive lookups (`Get("id")` and `Get("ID")` are equivalent)
- If a parameter doesn't exist, `Get()` returns an empty string
- `MessageParams` is a map (`map[string]string`) so if you want to check the original casing or process it in
your own way you can.

### Handler and Middleware Ordering

Handlers and middleware are executed in the order they are added to the router. This means that a handler added before another will always be checked for a match against the incoming message first regardless of the path pattern. This means you can easily predict how your handlers will be executed.

It also means that your handlers with more specific patterns should be added before any others that may share a common match.

```go
router.Bind("/album/:id(\\d{24})", GetAlbumByID)
router.Bind("/album/:name", GetAlbumsByName)
```

### PublicBind

`PublicBind()` marks routes as part of your public API, making them discoverable by API gateway frameworks for service registration and routing.

```go
router.Bind("/internal/metrics", internalHandler)        // Internal only
router.PublicBind("/api/users/:id", getUserHandler)      // Publicly discoverable
```

Gateway frameworks can call `router.RouteDescriptors()` to discover public routes.

**Eurus**, a WebSocket API gateway for microservices, is currently in development. Similar to how Zephyr provides HTTP API gateway functionality for Navaros-based services, Eurus will provide WebSocket API gateway capabilities for Velaros-based services. It will automatically discover public routes, handle client connections, and route messages to the appropriate backend services in your infrastructure.

### Route Introspection

Velaros provides methods for introspecting routes at runtime, useful for debugging, documentation generation, or dynamic routing scenarios.

**Lookup Handler Pattern:**

`router.Lookup(handler)` finds the pattern associated with a specific handler function. This is particularly useful in WebSocket-based routing where there are no HTTP methods:

```go
handler := func(ctx *velaros.Context) {
    ctx.Send(UserData{Name: "Alice"})
}

router.Bind("/users/:id", handler)

// Later, find what pattern this handler is bound to
pattern, found := router.Lookup(handler)
if found {
    log.Printf("Handler bound to: %s", pattern.String()) // "/users/:id"
}
```

This works with nested routers too - `Lookup()` recursively searches through mounted sub-routers to find handlers.

**Generate Paths from Patterns:**

`pattern.Path(params, wildcards)` generates URLs from route patterns by substituting parameters:

```go
pattern, _ := velaros.NewPattern("/users/:id/posts/:postID")

// Generate path with parameters
path, _ := pattern.Path(velaros.MessageParams{
    "id":     "123",
    "postID": "456",
}, nil)
// Result: "/users/123/posts/456"

// Works with wildcards too
pattern, _ := velaros.NewPattern("/files/*")
path, _ := pattern.Path(nil, []string{"documents/report.pdf"})
// Result: "/files/documents/report.pdf"
```

This is useful for:

- Generating client message paths dynamically
- Building reverse routing / URL generation
- Creating links in API responses
- Testing route patterns

**Combining Lookup and Path:**

```go
// Find the pattern for a handler, then generate a path
pattern, _ := router.Lookup(getUserHandler)
messagePath, _ := pattern.Path(velaros.MessageParams{"id": "123"}, nil)
// Send message to this path from client
```

## Connection Lifecycle

Velaros provides lifecycle middleware that executes at the beginning and end of a WebSocket connection's lifetime.

### UseOpen

`UseOpen()` registers handlers that execute immediately when a new WebSocket connection is established, before any messages are processed. This is useful for initialization, authentication, or setting up connection-level state.

```go
router.UseOpen(func(ctx *velaros.Context) {
    // Initialize connection state
    ctx.SetOnSocket("connectedAt", time.Now())

    // Generate and store session ID
    sessionID := uuid.NewString()
    ctx.SetOnSocket("sessionID", sessionID)

    log.Printf("New connection established: %s", sessionID)
})
```

### UseClose

`UseClose()` registers handlers that execute when a WebSocket connection is closing. This is useful for cleanup, logging, or notifying other systems about disconnections. If closed server-side, UseClose handlers can still send messages to the client before the connection closes.

```go
router.UseClose(func(ctx *velaros.Context) {
    // Retrieve connection info for logging
    sessionID, _ := ctx.GetFromSocket("sessionID")
    connectedAt, _ := ctx.GetFromSocket("connectedAt")

    duration := time.Since(connectedAt.(time.Time))
    log.Printf("Connection closed: %s (duration: %s)", sessionID, duration)

    // Send final message before close
    ctx.Send(GoodbyeMessage{Message: "Connection closing"})

    // Clean up resources
    cleanupUserSession(sessionID)
})
```

### Programmatic Connection Close

Handlers can programmatically close a connection using `ctx.Close()` or `ctx.CloseWithStatus()`. This stops message processing and executes all `UseClose` handlers before the connection is actually closed.

```go
router.Bind("/logout", func(ctx *velaros.Context) {
    username, _ := ctx.GetFromSocket("username")
    log.Printf("User %s logging out", username)

    // Send acknowledgment
    ctx.Send(LogoutResponse{Status: "logged out"})

    // Close the connection with normal closure status
    ctx.Close()
})

router.Bind("/kick", func(ctx *velaros.Context) {
    // Close with custom status and reason
    ctx.CloseWithStatus(velaros.StatusPolicyViolation, "Terms of service violation")
})
```

### Inspecting Close Information

`UseClose` handlers can inspect why a connection closed and whether it was server or client-initiated:

```go
router.UseClose(func(ctx *velaros.Context) {
    status, reason, source := ctx.CloseStatus()

    if source == velaros.ClientCloseSource {
        log.Printf("Client closed connection with status %d: %s", status, reason)
    } else {
        log.Printf("Server closed connection with status %d: %s", status, reason)
    }

    // Can still send farewell messages (for server-initiated closes)
    if source == velaros.ServerCloseSource {
        ctx.Send(GoodbyeMessage{Message: "Connection closed by server"})
    }
})
```

### WebSocket Status Codes

Velaros provides constants for all standard WebSocket close status codes defined in RFC 6455:

| Status Code | Constant | Description | When to Use |
|------------|----------|-------------|-------------|
| 1000 | `StatusNormalClosure` | Normal closure | Clean shutdown, user logout |
| 1001 | `StatusGoingAway` | Going away | Server shutting down, client navigating away |
| 1002 | `StatusProtocolError` | Protocol error | WebSocket protocol violation |
| 1003 | `StatusUnsupportedData` | Unsupported data | Received data type cannot be accepted |
| 1005 | `StatusNoStatusRcvd` | No status received | Reserved, should not be set explicitly |
| 1006 | `StatusAbnormalClosure` | Abnormal closure | Reserved, connection closed without close frame |
| 1007 | `StatusInvalidFramePayloadData` | Invalid frame payload | Message data inconsistent with type |
| 1008 | `StatusPolicyViolation` | Policy violation | Terms of service violation, abuse |
| 1009 | `StatusMessageTooBig` | Message too big | Message size exceeds limit |
| 1010 | `StatusMandatoryExtension` | Mandatory extension | Client expected extension not available |
| 1011 | `StatusInternalError` | Internal error | Server encountered unexpected condition |
| 1012 | `StatusServiceRestart` | Service restart | Server restarting |
| 1013 | `StatusTryAgainLater` | Try again later | Temporary condition, retry later |
| 1014 | `StatusBadGateway` | Bad gateway | Gateway or proxy error |
| 1015 | `StatusTLSHandshake` | TLS handshake | Reserved, TLS handshake failure |

**Common Usage:**

```go
// Normal logout
router.Bind("/auth/logout", func(ctx *velaros.Context) {
    ctx.Send(LogoutResponse{Status: "logged out"})
    ctx.CloseWithStatus(velaros.StatusNormalClosure, "User logged out")
})

// Policy violation (spam, abuse, etc.)
router.Bind("/moderate", func(ctx *velaros.Context) {
    if isSpamming(ctx) {
        ctx.CloseWithStatus(velaros.StatusPolicyViolation, "Spam detected")
        return
    }
})

// Server shutting down
router.UseClose(func(ctx *velaros.Context) {
    if isServerShutdown {
        ctx.CloseWithStatus(velaros.StatusGoingAway, "Server maintenance")
    }
})
```

**Note:** Status codes 1005, 1006, and 1015 are reserved and should not be set explicitly by applications. They are used by the WebSocket implementation itself.

### Lifecycle Hooks in Middleware Handlers

Handlers implementing the `OpenHandler` or `CloseHandler` interfaces can automatically register lifecycle hooks when used as middleware. This is particularly useful for routers or custom middleware that need to handle connection initialization and cleanup.

```go
// Custom middleware handler with lifecycle hooks
type AuthMiddleware struct {
    tokenValidator TokenValidator
}

func (m *AuthMiddleware) Handle(ctx *velaros.Context) {
    // Regular message handling - check if authenticated
    if !ctx.GetFromSocket("authenticated").(bool) {
        ctx.Send(ErrorResponse{Error: "unauthorized"})
        return
    }
    ctx.Next()
}

func (m *AuthMiddleware) HandleOpen(ctx *velaros.Context) {
    // Authenticate during connection open
    token := ctx.Headers().Get("Authorization")
    if user, err := m.tokenValidator.Validate(token); err == nil {
        ctx.SetOnSocket("user", user)
        ctx.SetOnSocket("authenticated", true)
    } else {
        ctx.SetOnSocket("authenticated", false)
    }
}

func (m *AuthMiddleware) HandleClose(ctx *velaros.Context) {
    // Cleanup on connection close
    if user, ok := ctx.GetFromSocket("user"); ok {
        log.Printf("User %v disconnected", user)
    }
}

// Use the middleware - lifecycle hooks are automatically registered
router.Use("/api/**", &AuthMiddleware{tokenValidator: validator})
```

When a handler implementing `OpenHandler` or `CloseHandler` is registered via `Use()`, Velaros automatically registers the `HandleOpen` and `HandleClose` methods as lifecycle handlers. This allows complex middleware (like auth handlers) to hook into the connection lifecycle without requiring manual registration via `UseOpen` and `UseClose`.

## Bidirectional Communication

Unlike HTTP, WebSocket connections are bidirectional - the server can send messages to clients at any time, not just in response to requests. Velaros provides several communication patterns to leverage this capability.

### Sending Messages

Use `Send()` to send messages to the client. When responding to a message that has an ID, `Send()` automatically uses that ID for the response:

```go
router.Bind("/process", func(ctx *velaros.Context) {
    var req ProcessRequest
    ctx.ReceiveInto(&req)

    // Send acknowledgment immediately
    ctx.Send(AckResponse{Status: "processing"})

    // Process the request
    result := performProcessing(req)

    // Send additional message with result
    ctx.Send(ProcessComplete{Result: result})
})
```

### Receiving Multiple Messages

Handlers can receive multiple messages from the same client by calling `Receive()` or `ReceiveInto()`. The first call to `ReceiveInto()` returns the trigger message (the message that invoked the handler). When that message includes an ID, an interceptor is automatically created so subsequent calls to `ReceiveInto()` receive additional messages from the client using that same ID:

```go
router.Bind("/conversation", func(ctx *velaros.Context) {
    // First ReceiveInto() gets the trigger message that invoked this handler
    var initialMsg ConversationStart
    if err := ctx.ReceiveInto(&initialMsg); err != nil {
        return
    }

    // Send greeting using data from trigger message
    ctx.Send(GreetingMessage{Text: "Hello " + initialMsg.Username + "! What's your name?"})

    // Now receive subsequent messages from the client
    var nameMsg NameMessage
    if err := ctx.ReceiveInto(&nameMsg); err != nil {
        return // Connection closed or timeout
    }

    // Continue conversation
    ctx.Send(GreetingMessage{Text: "Nice to meet you, " + nameMsg.Name})

    // Can receive multiple messages in a loop
    for {
        var msg ChatMessage
        if err := ctx.ReceiveInto(&msg); err != nil {
            return // Connection closed
        }

        ctx.Send(processMessage(msg))
    }
})
```

**Client-side example (JavaScript):**

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
const conversationId = 'conv-123';

// Initial message with ID and data
ws.send(JSON.stringify({
    id: conversationId,
    path: '/conversation',
    data: { Username: 'alice123' }
}));

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log('Server says:', msg.data.Text);

    // Send response with SAME ID for conversation to continue
    ws.send(JSON.stringify({
        id: conversationId,
        data: { Name: 'Alice' }
    }));
};
```

**Receive Methods:**

- `Receive() ([]byte, error)` - Returns raw message bytes
- `ReceiveInto(into any) error` - Unmarshals message into provided struct
- `ReceiveWithTimeout(timeout time.Duration) ([]byte, error)` - With custom timeout
- `ReceiveIntoWithTimeout(into any, timeout time.Duration) error` - With custom timeout
- `ReceiveWithContext(ctx context.Context) ([]byte, error)` - With context cancellation
- `ReceiveIntoWithContext(ctx context.Context, into any) error` - With context cancellation

**How it works:** The first call to `Receive()` or `ReceiveInto()` in a handler returns the trigger message (the message that invoked the handler). When that initial message includes an ID, Velaros automatically sets up an interceptor for that ID, so subsequent calls to `Receive()` or `ReceiveInto()` receive additional messages from the client using the same ID. The interceptor setup is completely transparent - you just keep calling `Receive()` or `ReceiveInto()` to get each message in sequence.

### Request and Response

The server can initiate requests to clients and wait for responses. `Request()` and `RequestInto()` are convenience methods that combine sending a message and receiving the response:

```go
type ConfirmRequest struct {
    Action string `json:"action"`
}

type ConfirmResponse struct {
    Confirmed bool `json:"confirmed"`
}

router.Bind("/delete/:id", func(ctx *velaros.Context) {
    id := ctx.Params().Get("id")

    // Request returns raw response bytes
    response, err := ctx.Request(ConfirmRequest{
        Action: "delete item " + id,
    })
    if err != nil {
        ctx.Send(ErrorResponse{Error: "request failed"})
        return
    }

    // Parse the response
    var confirm ConfirmResponse
    json.Unmarshal(response, &confirm)

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
        ctx.Send(ErrorResponse{Error: "request failed"})
        return
    }

    if confirm.Confirmed {
        deleteItem(id)
        ctx.Send(SuccessResponse{Status: "deleted"})
    }
})
```

**Note:** `Request()` and `RequestInto()` are simply convenience wrappers that call `Send()` followed by `Receive()` or `ReceiveInto()`. By default, they wait indefinitely for a response. Use the timeout or context variants to add timeouts.

### Request Timeouts and Cancellation

Control how long to wait for responses using timeout or context variants:

```go
// Custom timeout - waits up to 30 seconds for response
var response ConfirmResponse
err := ctx.RequestIntoWithTimeout(
    ConfirmRequest{Action: "approve"},
    &response,
    30 * time.Second,
)

// Context control - can cancel programmatically
cancelCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

err := ctx.RequestIntoWithContext(
    cancelCtx,
    ConfirmRequest{Action: "approve"},
    &response,
)
```

### Broadcasting Across Multiple Connections

For broadcasting messages to multiple clients or implementing pub/sub patterns across server instances, use an external message queue or pub/sub system. Velaros intentionally doesn't include built-in broadcasting because proper multi-instance deployment requires inter-process communication.

**Why External Systems?** When you scale your application horizontally (multiple server instances), WebSocket connections are distributed across different processes. Built-in broadcasting would only reach connections on the same instance. External systems like message queues, Redis Pub/Sub, or database events solve this by enabling communication between all server instances.

**Suitable Systems:**

- Message Queues: NATS, RabbitMQ, Kafka, AWS SQS
- Pub/Sub: Redis Pub/Sub, Google Pub/Sub
- Database Events: PostgreSQL LISTEN/NOTIFY, MongoDB Change Streams
- Event Buses: EventBridge, Apache Pulsar

**Pattern:**
Handlers that need to push events to clients should subscribe to a message queue and block, sending messages as events arrive.

> **⚠️ Important:** The handler in the example below **must block** and not return. Context objects are pooled and reused - if the handler returns while goroutines or callbacks still reference the context, those operations will fail with errors. The blocking pattern shown here is required, not optional.

```go
router.Bind("/notifications/subscribe", func(ctx *velaros.Context) {
    userID := getUserID(ctx)

    // Send acknowledgment
    ctx.Send(SubscribeResponse{Status: "subscribed"})

    // Subscribe to message queue for this user
    // This blocks and keeps the handler alive
    subscription := messageQueue.Subscribe("notifications."+userID)
    defer subscription.Unsubscribe()

    // Block and forward events as they arrive
    for {
        select {
        case msg := <-subscription.Messages():
            // Forward message to client
            if err := ctx.Send(Notification{
                Type: "notification",
                Data: msg.Data,
            }); err != nil {
                return // Connection closed
            }

        case <-ctx.Done():
            // Connection closed by client or server
            return
        }
    }
})

// Elsewhere in your application, broadcast by publishing to message queue
func broadcastNotification(notification Notification) {
    data, _ := json.Marshal(notification)
    // All subscribed handlers across all instances receive this
    messageQueue.Publish("notifications.*", data)
}

// Send to specific user
func sendToUser(userID string, notification Notification) {
    data, _ := json.Marshal(notification)
    messageQueue.Publish("notifications."+userID, data)
}
```

This pattern enables horizontal scaling - multiple Velaros instances can handle different WebSocket connections while all receiving the same broadcasts through your messaging system. The handler blocks while listening to the message queue, forwarding events to the client until the connection closes.

**Resource Cleanup Best Practices:**

When handlers block (like the broadcasting pattern above), proper cleanup is critical to avoid resource leaks:

1. **Always use `defer` for cleanup:**

   ```go
   subscription := messageQueue.Subscribe("topic")
   defer subscription.Unsubscribe()  // Ensures cleanup even on panic
   ```

2. **Always check `ctx.Done()` in blocking loops:**

   ```go
   for {
       select {
       case msg := <-subscription.Messages():
           ctx.Send(msg)
       case <-ctx.Done():
           return  // Critical: exits when connection closes
       }
   }
   ```

3. **Handle Send() errors:**

   ```go
   if err := ctx.Send(msg); err != nil {
       return  // Connection closed, stop processing
   }
   ```

**What happens without proper cleanup:**

- **Without `defer`:** Subscriptions remain active, leaking memory and connections
- **Without `ctx.Done()`:** Goroutines run forever, even after client disconnects
- **Without error handling:** Handler continues trying to send to closed connection

**The `ctx.Done()` channel:**

- Closes when the WebSocket connection closes (client or server initiated)
- Signals your handler to stop and cleanup
- Works with Go's `select` statement for clean shutdown
- Velaros Context implements Go's `context.Context` interface and can be passed to I/O operations (database queries, HTTP requests, etc.) to ensure they're cancelled when the connection closes
- `ctx.Done()` works like the standard library's cancellation pattern

## Advanced Usage

### Open and Close Handler Execution

Open and close handlers execute automatically in sequence - you don't need to call `ctx.Next()`. When a handler returns, the next handler is invoked automatically. 

**Post-Processing Pattern:**

Explicitly calling `ctx.Next()` allows you to perform work after all subsequent handlers complete:

```go
router.UseOpen(func(ctx *velaros.Context) {
    ctx.SetOnSocket("startTime", time.Now())
    
    ctx.Next() // Execute all subsequent handlers now
    
    // Runs after all downstream handlers complete
    duration := time.Since(ctx.MustGetFromSocket("startTime").(time.Time))
    log.Printf("Initialization took %v", duration)
})

router.UseOpen(func(ctx *velaros.Context) {
    ctx.SetOnSocket("sessionID", uuid.NewString())
})
```

Useful for timing, metrics, logging, or resource management that wraps initialization/cleanup.

**Note:** Handler chains cannot be short-circuited. Open handlers progress to the next handler unless the socket is closed. Close handlers always execute to completion.

### Authentication

Authentication state can be stored at the socket level so it persists across all messages from that connection. Use `UseOpen` to authenticate during the WebSocket handshake by reading headers (like Authorization tokens), then protect routes with middleware that checks authentication state.

```go
// Authenticate during connection setup using headers
router.UseOpen(func(ctx *velaros.Context) {
    authHeader := ctx.Headers().Get("Authorization")
    if authHeader == "" {
        ctx.CloseWithStatus(velaros.StatusPolicyViolation, "missing auth token")
        return
    }

    token := strings.TrimPrefix(authHeader, "Bearer ")
    user, err := validateToken(token)
    if err != nil {
        ctx.CloseWithStatus(velaros.StatusPolicyViolation, "invalid token")
        return
    }

    // Store authentication on socket (persists for connection lifetime)
    ctx.SetOnSocket("user", user)
    ctx.SetOnSocket("authenticated", true)
})

// Helper function to check if socket is authenticated
func isAuthenticated(ctx *velaros.Context) bool {
    authenticated, _ := ctx.GetFromSocket("authenticated")
    return authenticated == true
}

// Helper function to get authenticated user
func getUser(ctx *velaros.Context) (*User, bool) {
    user, ok := ctx.GetFromSocket("user")
    if !ok {
        return nil, false
    }
    return user.(*User), true
}

// Protected handler - uses helper to check auth
router.Bind("/user/profile", func(ctx *velaros.Context) {
    if !isAuthenticated(ctx) {
        ctx.Send(ErrorResponse{Error: "unauthorized"})
        return
    }

    user, _ := getUser(ctx)
    ctx.Send(UserProfile{Email: user.Email, Name: user.Name})
})
```

### Error Handling

Velaros provides robust error handling with automatic panic recovery. Errors and panics are captured and exposed through the context's `Error` field, allowing error-handling middleware to respond appropriately.

**Automatic Panic Recovery:**
Handler panics are automatically caught and converted to errors, preventing crashes. Stack traces are captured in `ctx.ErrorStack` for debugging.

**Error Handling Pattern:**
Use error-handling middleware that calls `ctx.Next()` first, then checks for errors after subsequent handlers execute.

```go
// Error handling middleware - runs after handlers
router.Use(func(ctx *velaros.Context) {
    ctx.Next()

    if ctx.Error != nil {
        // Log error with stack trace
        log.Printf("Error: %v\n%s", ctx.Error, ctx.ErrorStack)

        // Send error response to client
        ctx.Send(ErrorResponse{Error: ctx.Error.Error()})
    }
})

// Handlers can set errors explicitly
router.Bind("/data/process", func(ctx *velaros.Context) {
    var data ProcessData
    if err := ctx.ReceiveInto(&data); err != nil {
        ctx.Error = fmt.Errorf("invalid data: %w", err)
        return
    }

    if err := processData(data); err != nil {
        ctx.Error = fmt.Errorf("processing failed: %w", err)
        return
    }

    ctx.Send(SuccessResponse{Status: "processed"})
})

// Panics are automatically caught
router.Bind("/risky", func(ctx *velaros.Context) {
    // This panic will be caught and set as ctx.Error
    panic("something went wrong")
})
```

Once an error is set (either explicitly or via panic), subsequent handlers in the chain are skipped. Error-handling middleware can then examine `ctx.Error` and respond accordingly.

### API Gateway Integration

Velaros provides `PublicBind()` for marking routes as part of your public API, enabling API gateway frameworks to discover which routes your service handles for automatic service registration and routing. Gateway frameworks can call `router.RouteDescriptors()` to retrieve all public routes.

See the [PublicBind section](#publicbind) in Routing for details and examples. API gateway integration will be fully supported when **Eurus** (WebSocket API gateway) is released.

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
    ctx.Send(EchoResponse{Message: "echo"})
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

### Concurrency Model

**Each message is processed in its own goroutine.** This means multiple messages from the same client can be processed concurrently. Understanding this model is important for writing correct handlers:

**Key Points:**

1. **Concurrent Message Processing:** When a client sends multiple messages rapidly, they may be processed simultaneously in different goroutines.

2. **Thread-Safe Socket Storage:** Socket-level storage (`ctx.SetOnSocket()`, `ctx.GetFromSocket()`) is thread-safe and protected by mutexes. Multiple handlers can safely read/write socket state concurrently.

3. **Message Order Not Guaranteed:** Don't assume messages will be processed in the order they were sent. If order matters, implement your own sequencing.

4. **Blocking Handlers Are Fine:** When a handler blocks (e.g., subscribing to a message queue), other messages from the same client can still be processed in parallel.

```go
// This handler blocks, but other messages can still be processed
router.Bind("/notifications/subscribe", func(ctx *velaros.Context) {
    ctx.Send(SubscribeResponse{Status: "subscribed"})

    // This blocks indefinitely, but client can still send other messages
    // which will be processed in separate goroutines
    subscription := messageQueue.Subscribe("notifications")
    defer subscription.Unsubscribe()

    for {
        select {
        case msg := <-subscription.Messages():
            ctx.Send(Notification{Data: msg.Data})
        case <-ctx.Done():
            return
        }
    }
})

// This handler runs concurrently with the above
router.Bind("/user/profile", func(ctx *velaros.Context) {
    // Can be called while /notifications/subscribe is still running
    profile := getUserProfile(ctx)
    ctx.Send(profile)
})
```

**Best Practices:**

- **Handlers must block until all operations complete** - Context objects are pooled and reused. Spawning goroutines or callbacks that reference the context after the handler returns will cause errors (`"context cannot be used after handler returns"`). If your handler needs to send messages over time, it should block in a loop (see broadcasting examples above).
- Use socket-level storage for shared state (it's thread-safe)
- Don't assume message execution order
- Always check `ctx.Done()` in blocking loops to detect connection closure
- Handlers that block should use `select` with `ctx.Done()` for cleanup

## Use with Navaros

[Navaros](https://github.com/RobertWHurst/Navaros) is Velaros's sister HTTP framework, bringing the same routing patterns, middleware structure, and context-based approach to HTTP requests. Together, they provide a unified API for building applications that handle both HTTP and WebSocket traffic.

```go
import (
    "github.com/RobertWHurst/navaros"
    "github.com/RobertWHurst/velaros"
)

// Create HTTP router
httpRouter := navaros.NewRouter()
httpRouter.Get("/api/users/:id", func(ctx *navaros.Context) {
    ctx.Body = getUserByID(ctx.Params()["id"])
})

// Create WebSocket router
wsRouter := velaros.NewRouter()
wsRouter.Bind("/users/:id/status", func(ctx *velaros.Context) {
    // Stream real-time status updates...
})

// Mount WebSocket on HTTP router
httpRouter.Use("/ws", wsRouter.Middleware())

http.ListenAndServe(":8080", httpRouter)
```

**Key similarities:**
- Identical pattern syntax for routing (`/users/:id`, `/files/**`)
- Same middleware structure with `ctx.Next()` and pattern-scoped middleware
- Context-based storage (`ctx.Set()`, `ctx.Get()`)
- Consistent handler signatures and control flow

For complete documentation, see [Navaros on GitHub](https://github.com/RobertWHurst/Navaros).

## Testing

Test handlers using `httptest` and a WebSocket client:

```go
func TestEchoHandler(t *testing.T) {
    router := velaros.NewRouter()
    router.Use(json.Middleware())
    
    router.Bind("/echo", func(ctx *velaros.Context) {
        var req EchoRequest
        ctx.ReceiveInto(&req)
        ctx.Send(EchoResponse{Message: req.Message})
    })
    
    // Create test server
    server := httptest.NewServer(router)
    defer server.Close()
    
    // Connect WebSocket client
    ctx := context.Background()
    conn, _, _ := websocket.Dial(ctx, server.URL, nil)
    defer conn.Close(websocket.StatusNormalClosure, "")
    
    // Send message
    sendMessage(conn, "/echo", "123", EchoRequest{Message: "hello"})
    
    // Assert response
    var response EchoResponse
    readMessage(conn, &response)
    
    if response.Message != "hello" {
        t.Errorf("expected 'hello', got '%s'", response.Message)
    }
}
```

## Help Welcome

If you want to support this project by throwing me some coffee money it's greatly appreciated.

[![sponsor](https://img.shields.io/static/v1?label=Sponsor&message=%E2%9D%A4&logo=GitHub&color=%23fe8e86)](https://github.com/sponsors/RobertWHurst)

If you're interested in providing feedback or would like to contribute please feel free to do so. I recommend first opening an issue expressing your feedback or intent to contribute a change, from there we can consider your feedback or guide your contribution efforts. Any and all help is greatly appreciated since this is an open source effort after all.

Thank you!

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [Navaros](https://github.com/RobertWHurst/Navaros) - Lightweight HTTP framework for Go with flexible message routing and middleware - shares Velaros' routing patterns and API design philosophy
- [Zephyr](https://github.com/RobertWHurst/Zephyr) - Microservice framework built on Navaros with service discovery and fulling streaming HTTP all the way to the service.
- [Eurus](https://github.com/RobertWHurst/Eurus) - WebSocket API gateway framework for Velaros-based microservices
