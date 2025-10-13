# Velaros

A lightweight, flexible WebSocket framework for Go. Build real-time applications with powerful message routing, bidirectional communication, and composable middleware.

Velaros implements the standard `http.Handler` interface, so it works seamlessly with any Go HTTP router or framework - just mount it on a path like `/ws` and it handles the WebSocket upgrade automatically.

[![Go Reference](https://pkg.go.dev/badge/github.com/RobertWHurst/velaros.svg)](https://pkg.go.dev/github.com/RobertWHurst/velaros)
[![Go Report Card](https://goreportcard.com/badge/github.com/RobertWHurst/velaros)](https://goreportcard.com/report/github.com/RobertWHurst/velaros)

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Client Integration](#client-integration)
- [Core Concepts](#core-concepts)
  - [Message Format](#message-format)
  - [Context](#context)
  - [Context Lifecycle](#context-lifecycle)
  - [Middleware](#middleware)
- [Built-in Middleware](#built-in-middleware)
  - [JSON Middleware](#json-middleware)
  - [MessagePack Middleware](#messagepack-middleware)
  - [Protocol Buffers Middleware](#protocol-buffers-middleware)
  - [Set Middleware](#set-middleware)
- [Integration with HTTP Servers](#integration-with-http-servers)
- [Configuration](#configuration)
- [Routing](#routing)
- [Connection Lifecycle](#connection-lifecycle)
- [Bidirectional Communication](#bidirectional-communication)
- [Advanced Usage](#advanced-usage)
  - [Authentication](#authentication)
  - [Error Handling](#error-handling)
  - [API Gateway Integration](#api-gateway-integration)
- [Message Types](#message-types)
- [Performance](#performance)
- [Architecture](#architecture)
- [Examples](#examples)
- [Testing](#testing)
- [Help Welcome](#help-welcome)
- [License](#license)
- [Related Projects](#related-projects)

## Features

- 🚀 **High Performance** - Context pooling and efficient message routing
- 🔄 **Bidirectional** - Full duplex communication with Send, Reply, Request, and RequestInto patterns
- 🎯 **Powerful Patterns** - Flexible routing with parameters, wildcards, regex constraints, and modifiers
- 🔌 **Middleware** - Composable middleware for authentication, logging, and more
- 🔁 **Lifecycle Hooks** - UseOpen and UseClose middleware for connection initialization and cleanup
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

## Client Integration

**Note:** This guide uses JSON middleware for examples since it's human-readable and widely understood. Velaros also supports MessagePack (binary) and Protocol Buffers (schema-based). See the [Built-in Middleware](#built-in-middleware) section to learn about all available formats.

### Connecting from JavaScript/Browser

Velaros uses standard WebSocket protocol, so any WebSocket client can connect. Here's how to connect from a browser:

```javascript
// Connect to the WebSocket server
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
    console.log('Connected to Velaros server');

    // Send a message using JSON middleware format
    ws.send(JSON.stringify({
        path: '/chat/message',
        id: '123',  // Optional: include for request/reply correlation
        data: {
            username: 'Alice',
            text: 'Hello!'
        }
    }));
};

ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    console.log('Received:', message);
    // Message structure: { id: '123', data: {...} }
};

ws.onerror = (error) => {
    console.error('WebSocket error:', error);
};

ws.onclose = (event) => {
    console.log('Disconnected:', event.code, event.reason);
};
```

### Message Format with JSON Middleware

When using the JSON middleware (most common), clients must send messages in this structure:

```javascript
{
    "path": "/chat/message",     // Required: Routes to handler
    "id": "unique-id",           // Optional: For request/reply correlation
    "data": {                    // Your actual message payload
        "username": "Alice",
        "text": "Hello!"
    }
}
```

**Field Explanations:**

- **path**: Routes the message to the appropriate handler (like URL routing in HTTP)
- **id**: Optional message ID for correlating requests with replies. Include this if you expect a reply with the same ID.
- **data**: Your actual message content - can be any JSON-serializable data

Server responses use the same structure:

```javascript
{
    "id": "unique-id",    // Same ID if replying to a request
    "data": {             // Response payload
        "status": "success"
    }
}
```

### Client-Side Request/Reply Pattern

```javascript
// Client sends request and waits for reply
function sendRequest(path, data) {
    return new Promise((resolve, reject) => {
        const requestId = crypto.randomUUID();

        // Set up one-time listener for reply
        const handleMessage = (event) => {
            const message = JSON.parse(event.data);
            if (message.id === requestId) {
                ws.removeEventListener('message', handleMessage);
                resolve(message.data);
            }
        };

        ws.addEventListener('message', handleMessage);

        // Send request
        ws.send(JSON.stringify({
            path: path,
            id: requestId,
            data: data
        }));

        // Timeout after 5 seconds
        setTimeout(() => {
            ws.removeEventListener('message', handleMessage);
            reject(new Error('Request timeout'));
        }, 5000);
    });
}

// Usage
sendRequest('/user/profile', { userId: '123' })
    .then(profile => console.log('Profile:', profile))
    .catch(error => console.error('Error:', error));
```

### Server-Initiated Requests

The server can also initiate requests to the client. The client must reply with the same message ID:

```javascript
ws.onmessage = (event) => {
    const message = JSON.parse(event.data);

    // Check if this is a request from server (has id but we didn't send it)
    if (message.id && message.data.action === 'confirm') {
        // Server is asking for confirmation
        const confirmed = window.confirm(message.data.message);

        // Reply with same ID
        ws.send(JSON.stringify({
            id: message.id,  // IMPORTANT: Same ID to correlate
            data: { confirmed: confirmed }
        }));
    } else {
        // Regular message
        console.log('Received:', message.data);
    }
};
```

## Core Concepts

### Message Format

Velaros doesn't enforce any specific message format - it's entirely defined by the middleware you choose. Messages are raw bytes until middleware parses them and sets up marshallers/unmarshallers for your handlers.

The framework requires middleware to extract two pieces of information from inbound messages:

- **Message Path** - For routing messages to handlers
- **Message ID** - For request/reply correlation

Middleware does this by calling `ctx.SetMessagePath()` and `ctx.SetMessageID()`, then setting up `ctx.SetMessageUnmarshaler()` and `ctx.SetMessageMarshaller()` for encoding/decoding message data.

Message IDs are required for bidirectional request/reply patterns. When a client sends a message with an ID, handlers can use `Reply()` to send a response with the same ID. When handlers use `Request()` to query the client, the server generates an ID that the client must echo back in their response for proper correlation.

Velaros provides middleware for common formats: **JSON** (human-readable), **MessagePack** (binary, high-performance), and **Protocol Buffers** (schema-based, type-safe). You can also create custom middleware for other formats like CBOR, or even plain text/binary protocols. See the [Built-in Middleware](#built-in-middleware) section for details on each format.

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

**When to use Message vs Socket Storage:**

**Message Storage (`ctx.Set()`, `ctx.Get()`):**

- Request-specific data (request ID, start time, parsed request data)
- Data that should NOT persist across messages
- Temporary values for passing between middleware
- Automatically cleared after message processing completes

**Socket Storage (`ctx.SetOnSocket()`, `ctx.GetFromSocket()`):**

- Connection-specific data (user ID, session ID, auth tokens)
- Data that SHOULD persist for the entire connection lifetime
- Authentication state, user preferences, client metadata
- Thread-safe - can be accessed concurrently from multiple message handlers
- Cleared only when connection closes

**Examples:**

```go
// Authentication middleware stores user on socket
router.Use("/api/**", func(ctx *velaros.Context) {
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
    ctx.Reply(SuccessResponse{})
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

The Context provides access to HTTP headers from the initial WebSocket handshake request. This is useful for authentication, reading cookies, or accessing custom headers:

```go
router.UseOpen(func(ctx *velaros.Context) {
    // Access headers from the initial HTTP upgrade request
    headers := ctx.Headers()

    // Read Authorization header for JWT token
    authHeader := headers.Get("Authorization")
    if authHeader != "" {
        token := strings.TrimPrefix(authHeader, "Bearer ")
        if user, err := validateJWT(token); err == nil {
            ctx.SetOnSocket("user", user)
            ctx.SetOnSocket("authenticated", true)
        }
    }

    // Read cookies
    cookie := headers.Get("Cookie")
    if sessionID := extractSessionID(cookie); sessionID != "" {
        ctx.SetOnSocket("sessionID", sessionID)
    }

    // Custom headers
    clientVersion := headers.Get("X-Client-Version")
    ctx.SetOnSocket("clientVersion", clientVersion)

    log.Printf("New connection from %s", headers.Get("User-Agent"))
})

// Protected handler using authentication from headers
router.Bind("/admin/users", func(ctx *velaros.Context) {
    authenticated, _ := ctx.GetFromSocket("authenticated")
    if authenticated != true {
        ctx.Send(ErrorResponse{Error: "unauthorized"})
        return
    }

    // Proceed with admin operation
    ctx.Reply(AdminUsersResponse{Users: getUsers()})
})
```

**Note:** Headers are only available from the initial WebSocket handshake. Once the WebSocket connection is established, all communication uses WebSocket frames, not HTTP.

### Context Lifecycle

**Important:** Context objects are pooled and reused for performance. When a handler returns, its context is immediately returned to the pool and may be reused for a different message. This means **handlers must block until all operations using the context are complete**.

If you spawn a goroutine or set up a callback that references the context after the handler returns, those operations will fail with an error: `"context cannot be used after handler returns - handlers must block until all operations complete"`.

**Wrong - Don't do this:**

```go
// ❌ This will fail - goroutine uses context after handler returns
router.Bind("/subscribe", func(ctx *velaros.Context) {
    ctx.Reply(SubscribeResponse{Status: "subscribed"})

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
    ctx.Reply(SubscribeResponse{Status: "subscribed"})

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

**Message Structure:**

The JSON middleware expects messages in this format:

```json
{
    "path": "/user/create",
    "id": "optional-request-id",
    "data": {
        "username": "alice",
        "email": "alice@example.com"
    }
}
```

- **path** (string, required): Routes the message to the appropriate handler
- **id** (string, optional): For request/reply correlation - include when you expect a reply
- **data** (any, optional): The actual message payload that gets passed to `ctx.Unmarshal()`

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
    // Unmarshal extracts the "data" field from the message
    if err := ctx.Unmarshal(&user); err != nil {
        ctx.Send(ErrorResponse{Error: "invalid data"})
        return
    }

    // Process user...
    // Reply sends: {"id": "...", "data": {"username": "...", "email": "..."}}
    ctx.Reply(UserData{Username: user.Username, Email: user.Email})
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
    if err := ctx.Unmarshal(&req); err != nil {
        ctx.Send(ErrorResponse{Error: "invalid data"})
        return
    }

    // Process user...
    ctx.Reply(UserResponse{UserID: req.UserID, Name: req.Name})
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

- High-frequency trading or real-time systems
- Mobile apps with limited bandwidth
- IoT devices with constrained resources
- Game servers with many concurrent connections
- Any scenario where JSON performance is a bottleneck

### Protocol Buffers Middleware

Protocol Buffers (protobuf) is Google's language-neutral, platform-neutral serialization format. It provides strong typing, schema validation, and excellent performance with compact binary encoding.

**Why use Protocol Buffers:**

- **Type safety** - Schemas define exact message structure
- **Language-agnostic** - Use same .proto files across different languages
- **Efficient binary format** - Smaller and faster than JSON
- **Schema evolution** - Add fields without breaking existing clients
- **gRPC compatibility** - Standard format for microservices

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
    if err := ctx.Unmarshal(&req); err != nil {
        ctx.Send(&pb.ErrorResponse{Error: "invalid data"})
        return
    }

    // Process user...
    ctx.Reply(&pb.UserResponse{
        UserId:   123,
        Username: req.Username,
        Email:    req.Email,
    })
})
```

**How It Works:**

The middleware uses an internal envelope to add routing metadata (`id`, `path`) to your protobuf messages. This envelope is completely transparent - on the server side, you work directly with your generated protobuf types. The middleware handles wrapping and unwrapping automatically.

**Client-Side Usage (JavaScript):**

```javascript
import protobuf from 'protobufjs';

// Load your .proto definitions
const root = await protobuf.load('user.proto');
const CreateUserRequest = root.lookupType('myapp.CreateUserRequest');
const UserResponse = root.lookupType('myapp.UserResponse');

// Also load Velaros envelope
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

**Path Format:** Use gRPC-style paths like `/package.Service/Method` or custom paths like `/users/create`. Velaros routing works with any path format.

**Learn More:** [Official Protocol Buffers tutorial](https://protobuf.dev/getting-started/gotutorial/)

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

### Understanding Router Mounting

**Important:** The path you mount the router on becomes the WebSocket upgrade endpoint. Message paths inside the WebSocket connection are separate and independent.

```go
// Mount router at /api/ws
http.Handle("/api/ws", router)

// Clients connect to: ws://localhost:8080/api/ws
const ws = new WebSocket('ws://localhost:8080/api/ws');

// Once connected, message paths are internal to the WebSocket:
// These paths are NOT HTTP paths - they're message routes within the WebSocket
// Using JSON middleware format for this example:
ws.send(JSON.stringify({
    path: '/user/profile',  // This is a message path, not an HTTP path
    data: { userId: '123' }
}));
```

**Multiple Routers:**

You can mount multiple independent routers on different paths:

```go
publicRouter := velaros.NewRouter()
publicRouter.Bind("/chat", handlePublicChat)

adminRouter := velaros.NewRouter()
adminRouter.Bind("/users", handleAdminUsers)

// Mount on different paths
http.Handle("/ws/public", publicRouter)  // ws://host/ws/public
http.Handle("/ws/admin", adminRouter)    // ws://host/ws/admin

// Each router has independent handlers
// Messages sent to /ws/public use publicRouter's handlers
// Messages sent to /ws/admin use adminRouter's handlers
```

**Key Points:**

- HTTP mount path = WebSocket connection endpoint
- Message paths (inside WebSocket) = Routing within that connection
- Multiple routers = Multiple WebSocket endpoints, each independent
- Once WebSocket is established, all communication uses WebSocket frames (not HTTP)

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

### Accessing Route Parameters

Parameters extracted from route patterns are available via `ctx.Params()`, which returns a `MessageParams` type (a map[string]string). Parameter names are case-insensitive when retrieved.

**Example:**

```go
router.Bind("/users/:userID/posts/:postID", func(ctx *velaros.Context) {
    userID := ctx.Params().Get("userID")    // Gets "123"
    postID := ctx.Params().Get("postid")    // Case-insensitive: also works

    // Parameters are strings - convert as needed
    userIDInt, _ := strconv.Atoi(userID)

    posts := getUserPosts(userIDInt, postID)
    ctx.Reply(posts)
})

// Client sends: {path: "/users/123/posts/456", ...}
```

**Key Points:**

- Parameters are always strings - convert to other types as needed
- Parameter names are case-insensitive (`Get("id")` and `Get("ID")` are equivalent)
- If a parameter doesn't exist, `Get()` returns an empty string
- Parameters are extracted fresh for each message based on the matched route pattern

### Handler and Middleware Ordering

Handlers and middleware are executed in the order they are added to the router. This means that a handler added before another will always be checked for a match against the incoming message first regardless of the path pattern. This means you can easily predict how your handlers will be executed.

It also means that your handlers with more specific patterns should be added before any others that may share a common match.

```go
router.Bind("/album/:id(\\d{24})", GetAlbumByID)
router.Bind("/album/:name", GetAlbumsByName)
```

### PublicBind

For microservice architectures that use API gateways, `PublicBind()` allows you to mark routes as part of your public API. This enables gateway frameworks to discover which routes your service handles so they can route external requests appropriately.

```go
// Internal route - not announced to gateways
router.Bind("/internal/metrics", func(ctx *velaros.Context) {
    // Only accessible within your infrastructure
})

// Public route - announced to API gateways
router.PublicBind("/api/users/:id", func(ctx *velaros.Context) {
    // Discoverable by gateway frameworks
    userID := ctx.Params().Get("id")
    ctx.Reply(GetUser(userID))
})
```

The distinction between `Bind()` and `PublicBind()` helps separate your internal infrastructure routes from your external API surface. Gateway frameworks can call `router.RouteDescriptors()` to get a list of all public routes, enabling automatic service discovery and routing.

See the "API Gateway Integration" section in Advanced Usage for more details on using this pattern in microservice architectures.

## Connection Lifecycle

Velaros provides lifecycle middleware that executes at the beginning and end of a WebSocket connection's lifetime.

### UseOpen

`UseOpen()` registers middleware that executes immediately when a new WebSocket connection is established, before any messages are processed. This is useful for initialization, authentication, or setting up connection-level state.

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

`UseClose()` registers middleware that executes when a WebSocket connection is closing, after the message loop exits. This is useful for cleanup, logging, or notifying other systems about disconnections. UseClose middleware can still send messages to the client before the connection closes.

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

Handlers can programmatically close a connection using `ctx.Close()` or `ctx.CloseWithStatus()`. This stops message processing and executes all `UseClose` middleware before the connection is actually closed.

```go
router.Bind("/logout", func(ctx *velaros.Context) {
    username, _ := ctx.GetFromSocket("username")
    log.Printf("User %s logging out", username)

    // Send acknowledgment
    ctx.Reply(LogoutResponse{Status: "logged out"})

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

    if source == velaros.StatusSourceClient {
        log.Printf("Client closed connection with status %d: %s", status, reason)
    } else {
        log.Printf("Server closed connection with status %d: %s", status, reason)
    }

    // Can still send farewell messages (for server-initiated closes)
    if source == velaros.StatusSourceServer {
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
    ctx.Reply(LogoutResponse{Status: "logged out"})
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

### Multiple Lifecycle Handlers

You can register multiple `UseOpen` and `UseClose` middleware. They execute in order like regular middleware - call `ctx.Next()` to continue to the next handler in the chain.

```go
router.UseOpen(func(ctx *velaros.Context) {
    log.Println("Middleware 1: Connection opened")
    ctx.SetOnSocket("initialized", true)
    ctx.Next() // Continue to next UseOpen middleware
})

router.UseOpen(func(ctx *velaros.Context) {
    log.Println("Middleware 2: Initialize session")
    // No Next() call - this is the last middleware
})

router.UseClose(func(ctx *velaros.Context) {
    log.Println("Middleware 1: Cleanup session")
    ctx.Next() // Continue to next UseClose middleware
})

router.UseClose(func(ctx *velaros.Context) {
    log.Println("Middleware 2: Connection closed")
})
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
    ctx.Reply(SubscribeResponse{Status: "subscribed"})

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
- Same as `context.Context.Done()` - Velaros implements the standard interface

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

// Panics are automatically caught
router.Bind("/risky", func(ctx *velaros.Context) {
    // This panic will be caught and set as ctx.Error
    panic("something went wrong")
})
```

Once an error is set (either explicitly or via panic), subsequent handlers in the chain are skipped. Error-handling middleware can then examine `ctx.Error` and respond accordingly.

### API Gateway Integration

In microservice architectures, you often want to expose some WebSocket routes publicly through an API gateway while keeping others internal for service-to-service communication. Velaros supports this pattern through `PublicBind()` and route descriptors.

**The Problem:**

When building distributed systems, you need a way to:

- Separate public API routes from internal infrastructure routes
- Enable API gateways to discover which routes each service handles
- Route external WebSocket connections to the correct backend service

**The Solution:**
Use `PublicBind()` to mark routes that should be exposed through your gateway. The router collects these as route descriptors, which gateway frameworks can use for service discovery and routing.

```go
// Service A: User management service
userRouter := velaros.NewRouter()
userRouter.Use(json.Middleware())

// Public route - exposed through gateway
userRouter.PublicBind("/users/:id", func(ctx *velaros.Context) {
    userID := ctx.Params().Get("id")
    user := getUserByID(userID)
    ctx.Reply(user)
})

// Internal route - only accessible within infrastructure
userRouter.Bind("/internal/health", func(ctx *velaros.Context) {
    ctx.Reply(HealthStatus{OK: true})
})

// The gateway can discover public routes
descriptors := userRouter.RouteDescriptors()
// Returns: [RouteDescriptor{Pattern: "/users/:id"}]
// Note: /internal/health is NOT included
```

**Nested Routers:**
When you mount one router inside another using `PublicBind()`, the parent router collects all route descriptors from the child, with paths properly prefixed:

```go
// Create service-specific router
usersRouter := velaros.NewRouter()
usersRouter.PublicBind("/profile", GetUserProfile)
usersRouter.PublicBind("/settings", GetUserSettings)

// Mount at /users path
mainRouter := velaros.NewRouter()
mainRouter.PublicBind("/users/**", usersRouter)

// mainRouter.RouteDescriptors() returns:
// - /users/profile
// - /users/settings
```

**How Gateways Use This:**

Gateway frameworks like the upcoming Eurus can:

1. Connect to your service over a transport (NATS, local, etc.)
2. Call `RouteDescriptors()` to discover what routes the service handles
3. Build a routing table mapping external paths to backend services
4. Forward incoming WebSocket connections to the appropriate service

**When to Use:**

- Building microservice architectures with multiple WebSocket services
- Need API gateway for external traffic routing
- Want to hide internal infrastructure endpoints
- Deploying services that announce their capabilities dynamically

**When Not to Use:**

- Single-service deployments
- All routes are public (just use `Bind()`)
- Not using an API gateway framework

This pattern enables building scalable, distributed WebSocket systems where services can be discovered and routed automatically, without manual gateway configuration.

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
    ctx.Reply(SubscribeResponse{Status: "subscribed"})

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
    ctx.Reply(profile)
})
```

**Best Practices:**

- **Handlers must block until all operations complete** - Context objects are pooled and reused. Spawning goroutines or callbacks that reference the context after the handler returns will cause errors (`"context cannot be used after handler returns"`). If your handler needs to send messages over time, it should block in a loop (see broadcasting examples above).
- Use socket-level storage for shared state (it's thread-safe)
- Don't assume message execution order
- Always check `ctx.Done()` in blocking loops to detect connection closure
- Handlers that block should use `select` with `ctx.Done()` for cleanup

### Persistent Connections

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

**Note:** These examples use JSON middleware for clarity and simplicity. You can substitute MessagePack or Protocol Buffers middleware depending on your needs.

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

## Testing

Velaros handlers can be tested using Go's standard `httptest` package and the WebSocket client library.

### Basic Handler Test

```go
package main_test

import (
    "context"
    "encoding/json"
    "testing"
    "net/http/httptest"

    "github.com/RobertWHurst/velaros"
    jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
    "github.com/coder/websocket"
)

func TestEchoHandler(t *testing.T) {
    // Create router
    router := velaros.NewRouter()
    router.Use(jsonMiddleware.Middleware())

    // Add handler to test
    router.Bind("/echo", func(ctx *velaros.Context) {
        var req map[string]string
        if err := ctx.Unmarshal(&req); err != nil {
            t.Fatal(err)
        }
        ctx.Reply(map[string]string{"echo": req["message"]})
    })

    // Create test server
    server := httptest.NewServer(router)
    defer server.Close()

    // Connect WebSocket client
    ctx := context.Background()
    conn, _, err := websocket.Dial(ctx, server.URL, nil)
    if err != nil {
        t.Fatal(err)
    }
    defer conn.Close(websocket.StatusNormalClosure, "")

    // Send message
    message := map[string]any{
        "path": "/echo",
        "id":   "123",
        "data": map[string]string{"message": "hello"},
    }
    msgBytes, _ := json.Marshal(message)
    if err := conn.Write(ctx, websocket.MessageText, msgBytes); err != nil {
        t.Fatal(err)
    }

    // Read response
    _, responseBytes, err := conn.Read(ctx)
    if err != nil {
        t.Fatal(err)
    }

    var response map[string]any
    if err := json.Unmarshal(responseBytes, &response); err != nil {
        t.Fatal(err)
    }

    // Verify response
    if response["id"] != "123" {
        t.Errorf("expected id '123', got %v", response["id"])
    }

    data := response["data"].(map[string]any)
    if data["echo"] != "hello" {
        t.Errorf("expected echo 'hello', got %v", data["echo"])
    }
}
```

### Testing Lifecycle Hooks

```go
func TestUseOpenAndClose(t *testing.T) {
    router := velaros.NewRouter()
    router.Use(jsonMiddleware.Middleware())

    var openCalled, closeCalled bool

    router.UseOpen(func(ctx *velaros.Context) {
        openCalled = true
        ctx.SetOnSocket("initialized", true)
    })

    router.UseClose(func(ctx *velaros.Context) {
        closeCalled = true
        initialized, _ := ctx.GetFromSocket("initialized")
        if initialized != true {
            t.Error("expected initialized to be true")
        }
    })

    router.Bind("/test", func(ctx *velaros.Context) {
        ctx.Reply(map[string]string{"status": "ok"})
    })

    server := httptest.NewServer(router)
    defer server.Close()

    ctx := context.Background()
    conn, _, err := websocket.Dial(ctx, server.URL, nil)
    if err != nil {
        t.Fatal(err)
    }

    // Send a test message
    message := map[string]any{"path": "/test"}
    msgBytes, _ := json.Marshal(message)
    conn.Write(ctx, websocket.MessageText, msgBytes)

    // Close connection
    conn.Close(websocket.StatusNormalClosure, "test complete")

    // Give time for UseClose to execute
    time.Sleep(100 * time.Millisecond)

    if !openCalled {
        t.Error("UseOpen handler was not called")
    }
    if !closeCalled {
        t.Error("UseClose handler was not called")
    }
}
```

### Testing Server-Initiated Requests

```go
func TestServerInitiatedRequest(t *testing.T) {
    router := velaros.NewRouter()
    router.Use(jsonMiddleware.Middleware())

    router.Bind("/trigger", func(ctx *velaros.Context) {
        // Server asks client for confirmation
        var response map[string]bool
        err := ctx.RequestInto(
            map[string]string{"action": "confirm"},
            &response,
        )
        if err != nil {
            t.Fatal(err)
        }

        ctx.Reply(map[string]bool{"confirmed": response["confirmed"]})
    })

    server := httptest.NewServer(router)
    defer server.Close()

    ctx := context.Background()
    conn, _, err := websocket.Dial(ctx, server.URL, nil)
    if err != nil {
        t.Fatal(err)
    }
    defer conn.Close(websocket.StatusNormalClosure, "")

    // Handle server requests in background
    go func() {
        for {
            _, msgBytes, err := conn.Read(ctx)
            if err != nil {
                return
            }

            var msg map[string]any
            json.Unmarshal(msgBytes, &msg)

            // If this is a request from server, reply
            if msg["id"] != nil {
                response := map[string]any{
                    "id":   msg["id"],
                    "data": map[string]bool{"confirmed": true},
                }
                respBytes, _ := json.Marshal(response)
                conn.Write(ctx, websocket.MessageText, respBytes)
            }
        }
    }()

    // Send trigger message
    message := map[string]any{
        "path": "/trigger",
        "id":   "trigger-123",
    }
    msgBytes, _ := json.Marshal(message)
    conn.Write(ctx, websocket.MessageText, msgBytes)

    // Read final response
    _, responseBytes, _ := conn.Read(ctx)
    var response map[string]any
    json.Unmarshal(responseBytes, &response)

    data := response["data"].(map[string]any)
    if data["confirmed"] != true {
        t.Error("expected confirmed to be true")
    }
}
```

**Testing Tips:**

- Use `httptest.NewServer()` to create a test server
- Use `websocket.Dial()` from `github.com/coder/websocket` to connect
- Match your middleware format in tests (these examples use JSON: `{"path": "...", "id": "...", "data": {...}}`)
- For lifecycle hooks, add small delays (`time.Sleep`) to allow async operations to complete
- Test both happy path and error cases
- Use table-driven tests for testing multiple scenarios

## Help Welcome

If you want to support this project with coffee money, it's greatly appreciated.

[![sponsor](https://img.shields.io/static/v1?label=Sponsor&message=%E2%9D%A4&logo=GitHub&color=%23fe8e86)](https://github.com/sponsors/RobertWHurst)

If you're interested in providing feedback or would like to contribute, please feel free to do so. I recommend first [opening an issue][feature-request] expressing your feedback or intent to contribute a change. From there we can consider your feedback or guide your contribution efforts. Any and all help is greatly appreciated.

Thank you!

[feature-request]: https://github.com/RobertWHurst/Velaros/issues/new?template=feature_request.md

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [Navaros](https://github.com/RobertWHurst/Navaros) - HTTP framework for Go (companion project, shares similar API design)
- [Zephyr](https://github.com/TelemetryTV/Zephyr) - Microservice framework built on Navaros with service discovery and fulling streaming HTTP all the way to the service.
- Eurus - WebSocket API gateway framework (upcoming, integrates with Velaros via route descriptors)
