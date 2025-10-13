package velaros

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/coder/websocket"

	"github.com/RobertWHurst/navaros"
)

// BindType indicates the type of handler binding (regular message handler,
// open lifecycle handler, or close lifecycle handler).
type BindType int

const (
	// BindTypeBind indicates a regular message handler bound to a path pattern.
	BindTypeBind BindType = iota

	// BindTypeBindOpen indicates a lifecycle handler that runs when a connection opens.
	BindTypeBindOpen

	// BindTypeBindClose indicates a lifecycle handler that runs when a connection closes.
	BindTypeBindClose
)

// Router is the main WebSocket router that handles connection upgrades and
// message routing. It implements http.Handler for easy integration with Go's
// standard HTTP servers, and can also be used as middleware with Navaros.
//
// Router supports pattern-based message routing, middleware, lifecycle hooks,
// and route descriptors for API gateway integration.
type Router struct {
	routeDescriptorMap    map[string]bool
	routeDescriptors      []*RouteDescriptor
	firstHandlerNode      *HandlerNode
	lastHandlerNode       *HandlerNode
	firstOpenHandlerNode  *HandlerNode
	lastOpenHandlerNode   *HandlerNode
	firstCloseHandlerNode *HandlerNode
	lastCloseHandlerNode  *HandlerNode
	origins               []string
}

// NewRouter creates and returns a new WebSocket router.
func NewRouter() *Router {
	return &Router{}
}

// SetOrigins configures the allowed origin patterns for WebSocket connections.
// This is used for CORS-style origin validation during the WebSocket handshake.
// If not set, all origins are allowed (equivalent to []string{"*"}).
//
// Origin patterns support wildcards, for example:
//   - "https://example.com" - exact match
//   - "https://*.example.com" - subdomain wildcard
//   - "*" - allow all origins (default)
func (r *Router) SetOrigins(origins []string) {
	r.origins = origins
}

// Middleware returns a Navaros middleware function that handles WebSocket upgrade
// requests. This allows the router to be used as middleware in a Navaros HTTP router.
// If the request is a WebSocket upgrade, it handles the connection. Otherwise, it
// passes the request to the next handler in the Navaros chain.
func (r *Router) Middleware() navaros.HandlerFunc {
	return func(ctx *navaros.Context) {
		if r.isWebsocketUpgradeRequest(ctx.Request()) {
			navaros.CtxInhibitResponse(ctx)
			r.handleWebsocketConnection(ctx.ResponseWriter(), ctx.Request())
			return
		}
		ctx.Next()
	}
}

// ServeHTTP implements the http.Handler interface, allowing the router to be
// used directly with Go's standard HTTP server. It handles WebSocket upgrade
// requests and manages the connection lifecycle. If the request is not a
// WebSocket upgrade request, it returns a 400 Bad Request error.
func (r *Router) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if r.isWebsocketUpgradeRequest(req) {
		r.handleWebsocketConnection(res, req)
		return
	}
	res.WriteHeader(400)
	if _, err := res.Write([]byte("Bad Request. Expected websocket upgrade request")); err != nil {
		panic(err)
	}
}

// Handle implements the Handler interface, allowing the router to be used as
// a handler in another router's middleware chain. This enables mounting one
// router inside another for modular routing organization.
func (r *Router) Handle(ctx *Context) {
	subCtx := NewSubContextWithNode(ctx, r.firstHandlerNode)
	subCtx.Next()
	subCtx.free()
	if subCtx.currentHandlerNode != nil {
		ctx.Next()
	}
}

// Use registers middleware handlers that execute for all messages. Middleware
// can optionally be scoped to a specific path pattern by providing a path as
// the first argument. Handlers are executed in the order they are registered.
//
// Without a path, middleware runs for all messages:
//
//	router.Use(loggingMiddleware, authMiddleware)
//
// With a path, middleware only runs for matching messages:
//
//	router.Use("/api/**", authMiddleware)
//
// Routers can also be used as middleware to create modular sub-routers:
//
//	apiRouter := velaros.NewRouter()
//	router.Use("/api/**", apiRouter)
//
// Handlers must be of type Handler, HandlerFunc, or func(*Context).
func (r *Router) Use(handlers ...any) {
	mountPath := "/**"
	if len(handlers) != 0 {
		if customMountPath, ok := handlers[0].(string); ok {
			if !strings.HasSuffix(customMountPath, "/**") {
				customMountPath = strings.TrimSuffix(customMountPath, "/")
				customMountPath += "/**"
			}
			mountPath = customMountPath
			handlers = handlers[1:]
		}
	}

	r.bind(BindTypeBind, false, mountPath, handlers...)
}

// Bind registers handlers for messages matching the specified path pattern.
// Handlers are executed in order when a message's path matches the pattern.
// The path pattern supports parameters (:name), wildcards (*), and modifiers.
//
// Example patterns:
//
//	router.Bind("/users/list", listUsersHandler)
//	router.Bind("/users/:id", getUserHandler)
//	router.Bind("/files/**", fileHandler)
//
// Handlers must be of type Handler, HandlerFunc, or func(*Context).
// Panics if no handlers are provided or if handlers are of an invalid type.
func (r *Router) Bind(path string, handlers ...any) {
	r.bind(BindTypeBind, false, path, handlers...)
}

// PublicBind is like Bind but marks the route as part of the public API.
// This creates a route descriptor that can be discovered by API gateway
// frameworks for service discovery and routing. Use this for routes that
// should be exposed externally, and use Bind for internal-only routes.
//
// Example:
//
//	// Public API route - exposed through gateway
//	router.PublicBind("/api/users/:id", getUserHandler)
//
//	// Internal route - not exposed
//	router.Bind("/internal/health", healthHandler)
//
// Route descriptors can be retrieved via RouteDescriptors() for gateway integration.
func (r *Router) PublicBind(path string, handlers ...any) {
	r.bind(BindTypeBind, true, path, handlers...)
}

// UseOpen registers handlers that execute when a new WebSocket connection is
// established, before any messages are processed. This is useful for connection
// initialization, authentication checks, or setting up connection-level state.
//
// Example:
//
//	router.UseOpen(func(ctx *velaros.Context) {
//	    ctx.SetOnSocket("connectedAt", time.Now())
//	    ctx.SetOnSocket("sessionID", uuid.New())
//	})
//
// UseOpen handlers are executed in the order they are registered.
func (r *Router) UseOpen(handlers ...any) {
	r.bind(BindTypeBindOpen, false, "", handlers...)
}

// UseClose registers handlers that execute when a WebSocket connection is closing,
// after the message loop exits. This is useful for cleanup, logging, or notifying
// other systems about disconnections. UseClose handlers can still send messages
// to the client before the connection closes.
//
// Example:
//
//	router.UseClose(func(ctx *velaros.Context) {
//	    sessionID, _ := ctx.GetFromSocket("sessionID")
//	    log.Printf("Connection closed: %s", sessionID)
//	})
//
// UseClose handlers are executed in the order they are registered, for both
// server-initiated and client-initiated closures.
func (r *Router) UseClose(handlers ...any) {
	r.bind(BindTypeBindClose, false, "", handlers...)
}

// RouteDescriptors returns a list of all public route descriptors collected by
// this router. Route descriptors are generated when PublicBind is used, and
// they describe the routes that this router can handle. This is useful for API
// gateway frameworks that need to discover and route to WebSocket services.
func (r *Router) RouteDescriptors() []*RouteDescriptor {
	return r.routeDescriptors
}

func (r *Router) bind(bindType BindType, isPublic bool, path string, handlers ...any) {
	if len(handlers) == 0 {
		panic("no handlers provided")
	}

	for _, handler := range handlers {
		if _, ok := handler.(Handler); ok {
			continue
		} else if _, ok := handler.(HandlerFunc); ok {
			continue
		} else if _, ok := handler.(func(*Context)); ok {
			continue
		}

		panic("invalid handler type. Must be Handler, HandlerFunc, or " +
			"func(*Context). Got: " + reflect.TypeOf(handler).String())
	}

	var pattern *Pattern
	if bindType == BindTypeBind {
		var err error
		pattern, err = NewPattern(path)
		if err != nil {
			panic("invalid route pattern \"" + path + "\": " + err.Error())
		}

		hasAddedOwnRouteDescriptor := false
		for _, handler := range handlers {
			if routerHandler, ok := handler.(RouterHandler); ok {
				for _, routeDescriptor := range routerHandler.RouteDescriptors() {
					mountPath := strings.TrimSuffix(path, "/**")
					subPattern, err := NewPattern(mountPath + routeDescriptor.Pattern.String())
					if err != nil {
						panic("invalid route pattern \"" + mountPath + routeDescriptor.Pattern.String() + "\": " + err.Error())
					}
					r.addRouteDescriptor(subPattern)
				}
			} else if isPublic && !hasAddedOwnRouteDescriptor {
				r.addRouteDescriptor(pattern)
				hasAddedOwnRouteDescriptor = true
			}
		}
	}

	nextHandlerNode := &HandlerNode{
		BindType: bindType,
		Pattern:  pattern,
		Handlers: handlers,
	}

	switch bindType {
	case BindTypeBind:
		if r.firstHandlerNode == nil {
			r.firstHandlerNode = nextHandlerNode
			r.lastHandlerNode = nextHandlerNode
		} else {
			r.lastHandlerNode.Next = nextHandlerNode
			r.lastHandlerNode = nextHandlerNode
		}
	case BindTypeBindOpen:
		if r.firstOpenHandlerNode == nil {
			r.firstOpenHandlerNode = nextHandlerNode
			r.lastOpenHandlerNode = nextHandlerNode
		} else {
			r.lastOpenHandlerNode.Next = nextHandlerNode
			r.lastOpenHandlerNode = nextHandlerNode
		}
	case BindTypeBindClose:
		if r.firstCloseHandlerNode == nil {
			r.firstCloseHandlerNode = nextHandlerNode
			r.lastCloseHandlerNode = nextHandlerNode
		} else {
			r.lastCloseHandlerNode.Next = nextHandlerNode
			r.lastCloseHandlerNode = nextHandlerNode
		}
	}
}

func (r *Router) addRouteDescriptor(pattern *Pattern) {
	path := pattern.String()
	if r.routeDescriptorMap == nil {
		r.routeDescriptorMap = map[string]bool{}
	}
	if r.routeDescriptors == nil {
		r.routeDescriptors = []*RouteDescriptor{}
	}
	if _, ok := r.routeDescriptorMap[path]; ok {
		return
	}
	r.routeDescriptorMap[path] = true
	r.routeDescriptors = append(r.routeDescriptors, &RouteDescriptor{
		Pattern: pattern,
	})
}

func (r *Router) isWebsocketUpgradeRequest(req *http.Request) bool {
	return req.Header.Get("Upgrade") == "websocket"
}

func (r *Router) handleWebsocketConnection(res http.ResponseWriter, req *http.Request) {
	origins := r.origins
	if len(origins) == 0 {
		origins = []string{"*"}
	}

	conn, err := websocket.Accept(res, req, &websocket.AcceptOptions{
		OriginPatterns: origins,
	})
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "failed to accept websocket connection")
		panic(err)
	}

	socket := NewSocket(req.Header, conn)

	if r.firstOpenHandlerNode != nil {
		openCtx := NewContextWithNode(socket, inboundMessageFromPool(), r.firstOpenHandlerNode)
		openCtx.Next()
		openCtx.free()
	}

	for socket.handleNextMessageWithNode(r.firstHandlerNode) {
	}

	if r.firstCloseHandlerNode != nil {
		closeCtx := NewContextWithNode(socket, inboundMessageFromPool(), r.firstCloseHandlerNode)
		closeCtx.Next()
		closeCtx.free()
	}

	socket.closeMx.Lock()
	defer socket.closeMx.Unlock()

	_ = conn.Close(socket.closeStatus, socket.closeReason)
}
