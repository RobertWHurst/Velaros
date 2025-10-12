package velaros

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/coder/websocket"

	"github.com/RobertWHurst/navaros"
)

type Router struct {
	routeDescriptorMap map[string]bool
	routeDescriptors   []*RouteDescriptor
	firstHandlerNode   *HandlerNode
	lastHandlerNode    *HandlerNode
}

func NewRouter() *Router {
	return &Router{}
}

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

func (r *Router) Handle(ctx *Context) {
	subCtx := NewSubContextWithNode(ctx, r.firstHandlerNode)
	subCtx.Next()
	subCtx.free()
	if subCtx.currentHandlerNode != nil {
		ctx.Next()
	}
}

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

	r.bind(false, mountPath, handlers...)
}

func (r *Router) Bind(path string, handlers ...any) {
	r.bind(false, path, handlers...)
}

func (r *Router) PublicBind(path string, handlers ...any) {
	r.bind(true, path, handlers...)
}

func (r *Router) bind(isPublic bool, path string, handlers ...any) {
	if len(handlers) == 0 {
		panic("no handlers provided")
	}

	pattern, err := NewPattern(path)
	if err != nil {
		panic(err)
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

	hasAddedOwnRouteDescriptor := false
	for _, handler := range handlers {
		if routerHandler, ok := handler.(RouterHandler); ok {
			for _, routeDescriptor := range routerHandler.RouteDescriptors() {
				mountPath := strings.TrimSuffix(path, "/**")
				subPattern, err := NewPattern(mountPath + routeDescriptor.Pattern.String())
				if err != nil {
					panic(err)
				}
				r.addRouteDescriptor(subPattern)
			}
		} else if isPublic && !hasAddedOwnRouteDescriptor {
			r.addRouteDescriptor(pattern)
			hasAddedOwnRouteDescriptor = true
		}
	}

	nextHandlerNode := &HandlerNode{
		Pattern:  pattern,
		Handlers: handlers,
	}

	if r.firstHandlerNode == nil {
		r.firstHandlerNode = nextHandlerNode
		r.lastHandlerNode = nextHandlerNode
	} else {
		r.lastHandlerNode.Next = nextHandlerNode
		r.lastHandlerNode = nextHandlerNode
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
	conn, err := websocket.Accept(res, req, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		conn.Close(websocket.StatusInternalError, "failed to accept websocket connection")
		panic(err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	socket := newSocket(req.Header, conn)
	defer socket.close()

	for {
		if !socket.handleNextMessageWithNode(r.firstHandlerNode) {
			break
		}
	}
}
