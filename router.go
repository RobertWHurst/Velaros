package velaros

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/coder/websocket"

	"github.com/RobertWHurst/navaros"
)

type BindType int

const (
	BindTypeBind BindType = iota
	BindTypeBindOpen
	BindTypeBindClose
)

type Router struct {
	routeDescriptorMap map[string]bool
	routeDescriptors   []*RouteDescriptor
	firstHandlerNode   *HandlerNode
	lastHandlerNode    *HandlerNode
	firstOpenHandlerNode   *HandlerNode
	lastOpenHandlerNode    *HandlerNode
	firstCloseHandlerNode  *HandlerNode
	lastCloseHandlerNode   *HandlerNode
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

	r.bind(BindTypeBind, false, mountPath, handlers...)
}

func (r *Router) Bind(path string, handlers ...any) {
	r.bind(BindTypeBind, false, path, handlers...)
}

func (r *Router) PublicBind(path string, handlers ...any) {
	r.bind(BindTypeBind, true, path, handlers...)
}

func (r *Router) UseOpen(handlers ...any) {
	r.bind(BindTypeBindOpen, false, "", handlers...)
}

func (r *Router) UseClose(handlers ...any) {
	r.bind(BindTypeBindClose, false, "", handlers...)
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

	if r.firstOpenHandlerNode != nil {
		openCtx := NewContextWithNode(socket, &InboundMessage{}, r.firstOpenHandlerNode)
		openCtx.Next()
		openCtx.free()
	}

	for {
		if !socket.handleNextMessageWithNode(r.firstHandlerNode) {
			break
		}
	}

	if r.firstCloseHandlerNode != nil {
		closeCtx := NewContextWithNode(socket, &InboundMessage{}, r.firstCloseHandlerNode)
		closeCtx.Next()
		closeCtx.free()
	}
}
