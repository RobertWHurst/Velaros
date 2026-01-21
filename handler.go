package velaros

// Handler is a handler object interface. Any object that implements this
// interface can be used as a handler in a handler chain.
type Handler interface {
	Handle(ctx *Context)
}

// OpenHandler extends Handler to support connection open lifecycle hooks.
// When used as middleware (via Use), handlers implementing this interface will
// automatically have their HandleOpen method registered as UseOpen middleware.
type OpenHandler interface {
	HandleOpen(ctx *Context)
}

// CloseHandler extends Handler to support connection close lifecycle hooks.
// When used as middleware (via Use), handlers implementing this interface will
// automatically have their HandleClose method registered as UseClose middleware.
type CloseHandler interface {
	HandleClose(ctx *Context)
}

// HandlerFunc is a function adapter that allows ordinary functions to be used as
// handlers. This is the most common way to define handlers. For stateful handlers or
// those needing HandleOpen/HandleClose, implement the Handler interface instead.
type HandlerFunc func(ctx *Context)

// RouterHandler is implemented by routers to enable composition and nesting. When a
// RouterHandler is bound as middleware, the parent router collects its route descriptors
// for API discovery and can perform reverse routing through nested routers via Lookup.
// This allows building modular applications with sub-routers.
type RouterHandler interface {
	RouteDescriptors() []*RouteDescriptor
	Handle(ctx *Context)
	Lookup(handlerOrTransformer any) (*Pattern, bool)
}
