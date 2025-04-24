package localconnection

func (c *Connection) Ignore(interplexerID string, socketID string, messageID string) error {
	handler, ok := c.ignoreHandlers[interplexerID]
	if !ok {
		return nil
	}
	handler(socketID, messageID)
	return nil
}

func (c *Connection) BindIgnore(interplexerID string, handler func(socketID string, messageID string)) error {
	c.ignoreHandlers[interplexerID] = handler
	return nil
}

func (c *Connection) UnbindIgnore(interplexerID string) error {
	delete(c.ignoreHandlers, interplexerID)
	return nil
}