package velaros

type HandlerNode struct {
	BindType BindType
	Pattern  *Pattern
	Handlers []any
	Next     *HandlerNode
}

func (n *HandlerNode) tryMatch(ctx *Context) bool {
	if n.Pattern == nil {
		return true
	}
	return n.Pattern.MatchInto(ctx.Path(), &ctx.params)
}
