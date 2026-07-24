package velaros

// RouteOption is an interface for options that can be passed alongside
// handlers when binding routes. Route options are extracted before handler
// validation and do not participate in the handler chain.
type RouteOption interface {
	isRouteOption()
}

// MetadataOption is a RouteOption that carries arbitrary metadata to be
// attached to the route descriptor.
type MetadataOption struct {
	value any
}

func (MetadataOption) isRouteOption() {}

// WithMetadata creates a RouteOption that attaches arbitrary metadata to
// the route descriptor. This metadata can be used by gateways and
// middleware to implement features like rate limiting, auth requirements,
// etc.
func WithMetadata(value any) MetadataOption {
	return MetadataOption{value: value}
}
