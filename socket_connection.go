package velaros

import (
	"context"

	"github.com/coder/websocket"
)

// WebSocketConnection is a SocketConnection implementation that wraps
// github.com/coder/websocket.Conn. This is the default connection type used by the
// router when handling HTTP WebSocket upgrades.
type WebSocketConnection struct {
	webSocketConnection *websocket.Conn
}

var _ SocketConnection = &WebSocketConnection{}

// NewWebSocketConnection creates a WebSocketConnection from a github.com/coder/websocket.Conn.
// This is used internally by the router. Most applications don't need to call this directly
// unless implementing custom connection handling with Router.HandleConnection.
func NewWebSocketConnection(websocketConnection *websocket.Conn) *WebSocketConnection {
	return &WebSocketConnection{
		webSocketConnection: websocketConnection,
	}
}

// Read reads the next message from the WebSocket connection. Blocks until a message
// arrives or an error occurs. Implements SocketConnection.Read.
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

// Write sends a message to the WebSocket connection. Implements SocketConnection.Write.
func (c *WebSocketConnection) Write(ctx context.Context, msg *SocketMessage) error {
	return c.webSocketConnection.Write(ctx, msg.Type, msg.Data)
}

// Close closes the WebSocket connection with the given status code and reason.
// Implements SocketConnection.Close.
func (c *WebSocketConnection) Close(status Status, reason string) error {
	return c.webSocketConnection.Close(websocket.StatusCode(status), reason)
}
