package velaros

// Handler is a handler object interface. Any object that implements this
// interface can be used as a handler in a handler chain.
type Handler interface {
	Handle(ctx *Context)
}

// HandlerFunc is a function that can be used as a handler with Velaros.
type HandlerFunc func(ctx *Context)

// RouterHandler is handled nearly identically to a Handler, but it also
// provides a list of route descriptors which are collected by the router.
// These will be merged with the other route descriptors already collected.
// This use for situation where a handler may do more sub-routing, and the
// allows the handler to report the sub-routes to the router, rather than
// it's base path.
type RouterHandler interface {
	RouteDescriptors() []*RouteDescriptor
	Handle(ctx *Context)
}
