package scramjet

type HandlerChain struct {
	firstNode *HandlerNode
	lastNode  *HandlerNode
}

func (c *HandlerChain) MaybeMatchPath(path string) *Params {
	return nil
}
