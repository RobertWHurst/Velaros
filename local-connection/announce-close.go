package localconnection

func (c *Connection) AnnounceSocketClose(interplexerID string, socketID string) error {
	for _, handler := range c.announceCloseHandlers {
		handler(interplexerID, socketID)
	}
	return nil
}

func (c *Connection) BindSocketCloseAnnounce(handler func(interplexerID string, socketID string)) error {
	c.announceCloseHandlers = append(c.announceCloseHandlers, handler)
	return nil
}

func (c *Connection) UnbindSocketCloseAnnounce() error {
	c.announceCloseHandlers = []func(interplexerID string, socketID string){}
	return nil
}
