package localconnection

import "time"

func (c *Connection) Intercept(interplexerID string, socketID string, messageID string, timeout time.Duration) error {
	handler, ok := c.interceptHandlers[interplexerID]
	if !ok {
		return nil
	}
	handler(socketID, messageID, timeout)
	return nil
}

func (c *Connection) BindIntercept(interplexerID string, handler func(socketID string, messageID string, timeout time.Duration)) error {
	c.interceptHandlers[interplexerID] = handler
	return nil
}

func (c *Connection) UnbindIntercept(interplexerID string) error {
	delete(c.interceptHandlers, interplexerID)
	return nil
}