package localconnection

func (c *Connection) Intercepted(interplexerID string, socketID string, messageID string, message []byte) error {
	handler, ok := c.interceptedHandlers[interplexerID]
	if !ok {
		return nil
	}
	handler(socketID, messageID, message)
	return nil
}

func (c *Connection) BindIntercepted(interplexerID string, handler func(socketID string, messageID string, message []byte)) error {
	c.interceptedHandlers[interplexerID] = handler
	return nil
}

func (c *Connection) UnbindIntercepted(interplexerID string) error {
	delete(c.interceptedHandlers, interplexerID)
	return nil
}