package velaros

// Transformer is a special type of handler object that can be used to
// transform the context before and after handlers have processed the request.
// This is most useful for modifying or re-encoding the request and response
// bodies.
type Transformer interface {
	TransformRequest(ctx *Context)
	TransformResponse(ctx *Context)
}
