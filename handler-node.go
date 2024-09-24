package scramjet

type HandlerNode struct {
	Pattern                 *Pattern
	HandlersAndTransformers []any
	Next                    *HandlerNode
}

func (n *HandlerNode) tryMatch(ctx *Context) bool {
	if n.Pattern == nil {
		return true
	}
	return n.Pattern.MatchInto(ctx.path, &ctx.params)
}
