package natsconnection

func (c *Connection) Dispatch(interplexerID string, socketID string, message []byte) error {
	return nil
}

func (c *Connection) BindDispatch(interplexerID string, handler func(socketID string, message []byte) bool) error {
	return nil
}
