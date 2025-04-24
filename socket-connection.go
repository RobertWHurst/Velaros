package velaros

import (
	"context"
	"fmt"

	"github.com/coder/websocket"
)

type SocketConnection interface {
	Write(ctx context.Context, data []byte) error
	Read(ctx context.Context) ([]byte, error)
}

type websocketConn struct {
	conn *websocket.Conn
}

var _ SocketConnection = &websocketConn{}

func (c *websocketConn) Write(ctx context.Context, messageData []byte) error {
	return c.conn.Write(ctx, websocket.MessageText, messageData)
}

func (c *websocketConn) Read(ctx context.Context) ([]byte, error) {
	messageKind, messageData, err := c.conn.Read(ctx)
	if err == nil && messageKind != websocket.MessageText {
		return nil, fmt.Errorf("Message type %v is not supported", messageKind)
	}
	return messageData, err
}
