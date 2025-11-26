package velaros

import (
	"context"

	"github.com/coder/websocket"
)

type WebSocketConnection struct {
	webSocketConnection *websocket.Conn
}

var _ SocketConnection = &WebSocketConnection{}

func NewWebSocketConnection(websocketConnection *websocket.Conn) *WebSocketConnection {
	return &WebSocketConnection{
		webSocketConnection: websocketConnection,
	}
}

func (c *WebSocketConnection) Read(ctx context.Context) (*SocketMessage, error) {
	messageType, data, err := c.webSocketConnection.Read(ctx)
	if err != nil {
		return nil, err
	}

	return &SocketMessage{
		Type:    messageType,
		RawData: data,
	}, nil
}

func (c *WebSocketConnection) Write(ctx context.Context, msg *SocketMessage) error {
	return c.webSocketConnection.Write(ctx, msg.Type, msg.Data)
}

func (c *WebSocketConnection) Close(status Status, reason string) error {
	return c.webSocketConnection.Close(websocket.StatusCode(status), reason)
}
