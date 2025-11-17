package velaros

// CtxSocket retrieves the Socket associated with the given Context.
// This function is for frameworks to be able to grab the underlying Socket
// and shouldn't be used in most cases.
func CtxSocket(ctx *Context) *Socket {
	return ctx.socket
}

// CtxFree releases the resources associated with the given Context.
// This function is for frameworks to be able to free Contexts they create
// and shouldn't be used in most cases.
func CtxFree(ctx *Context) {
	ctx.free()
}

func CtxAssociatedValues(ctx *Context) map[string]any {
	return ctx.associatedValues
}

func CtxSocketAssociatedValues(ctx *Context) map[string]any {
	return ctx.socket.associatedValues
}
