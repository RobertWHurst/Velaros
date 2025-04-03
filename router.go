package velaros

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/coder/websocket"

	"github.com/RobertWHurst/navaros"
)

type MessageDecoder func([]byte) (*InboundMessage, error)
type MessageEncoder func(*OutboundMessage) ([]byte, error)

func DefaultMessageDecoder(msg []byte) (*InboundMessage, error) {
	message := &InboundMessage{}
	err := json.Unmarshal(msg, message)
	return message, err
}

func DefaultMessageEncoder(message *OutboundMessage) ([]byte, error) {
	return json.Marshal(message)
}

type Router struct {
	routeDescriptorMap map[string]bool
	routeDescriptors   []*RouteDescriptor
	firstHandlerNode   *HandlerNode
	lastHandlerNode    *HandlerNode
	interplexer        *interplexer
	MessageDecoder     MessageDecoder
	MessageEncoder     MessageEncoder
}

var _ RouterHandler = &Router{}

func NewRouter() *Router {
	return &Router{
		interplexer:    newInterplexer(),
		MessageEncoder: DefaultMessageEncoder,
		MessageDecoder: DefaultMessageDecoder,
	}
}

func (r *Router) RouteDescriptors() []*RouteDescriptor {
	return r.routeDescriptors
}

func (r *Router) ConnectInterplexer(connection InterplexerConnection) error {
	return r.interplexer.setConnection(connection)
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

func (r *Router) Use(handlersAndTransformers ...any) {
	mountPath := "/**"
	if len(handlersAndTransformers) != 0 {
		if customMountPath, ok := handlersAndTransformers[0].(string); ok {
			if !strings.HasSuffix(customMountPath, "/**") {
				customMountPath = strings.TrimSuffix(customMountPath, "/")
				customMountPath += "/**"
			}
			mountPath = customMountPath
			handlersAndTransformers = handlersAndTransformers[1:]
		}
	}

	r.bind(false, mountPath, handlersAndTransformers...)
}

func (r *Router) Bind(path string, handlersAndTransformers ...any) {
	r.bind(false, path, handlersAndTransformers...)
}

func (r *Router) PublicBind(path string, handlersAndTransformers ...any) {
	r.bind(true, path, handlersAndTransformers...)
}

func (r *Router) bind(isPublic bool, path string, handlersAndTransformers ...any) {
	if len(handlersAndTransformers) == 0 {
		panic("no handlers or transformers provided")
	}

	pattern, err := NewPattern(path)
	if err != nil {
		panic(err)
	}

	for _, handlerOrTransformer := range handlersAndTransformers {
		if _, ok := handlerOrTransformer.(Transformer); ok {
			continue
		} else if _, ok := handlerOrTransformer.(Handler); ok {
			continue
		} else if _, ok := handlerOrTransformer.(HandlerFunc); ok {
			continue
		} else if _, ok := handlerOrTransformer.(func(*Context)); ok {
			continue
		}

		panic("invalid handler type. Must be a Transformer, Handler, or " +
			"HandlerFunc. Got: " + reflect.TypeOf(handlerOrTransformer).String())
	}

	hasAddedOwnRouteDescriptor := false
	for _, handlerOrTransformer := range handlersAndTransformers {
		if routerHandler, ok := handlerOrTransformer.(RouterHandler); ok {
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
		Pattern:                 pattern,
		HandlersAndTransformers: handlersAndTransformers,
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

	socket := NewSocket(&websocketConn{conn: conn}, r.interplexer, r.MessageDecoder, r.MessageEncoder)
	defer socket.close()

	for {
		if !socket.handleNextMessageWithNode(r.firstHandlerNode) {
			break
		}
	}
}
