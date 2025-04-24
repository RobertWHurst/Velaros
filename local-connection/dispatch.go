package localconnection

func (c *Connection) Dispatch(interplexerID string, socketID string, message []byte) error {
	handler, ok := c.dispatchHandlers[interplexerID]
	if !ok {
		return nil
	}
	handler(socketID, message)
	return nil
}

func (c *Connection) BindDispatch(interplexerID string, handler func(socketID string, message []byte)) error {
	c.dispatchHandlers[interplexerID] = handler
	return nil
}

func (c *Connection) UnbindDispatch(interplexerID string) error {
	delete(c.dispatchHandlers, interplexerID)
	return nil
}
