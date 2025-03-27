package localconnection

func (c *Connection) AnnounceSocketOpen(interplexerID string, socketID string) error {
	for _, handler := range c.announceOpenHandlers {
		handler(interplexerID, socketID)
	}
	return nil
}

func (c *Connection) BindSocketOpenAnnounce(handler func(interplexerID string, socketID string)) error {
	c.announceOpenHandlers = append(c.announceOpenHandlers, handler)
	return nil
}

func (c *Connection) UnbindSocketOpenAnnounce() error {
	c.announceOpenHandlers = []func(interplexerID string, socketID string){}
	return nil
}
