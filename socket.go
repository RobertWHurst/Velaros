package scramjet

import (
	"context"

	"github.com/coder/websocket"
)

type Socket struct {
	connection       *websocket.Conn
	MessageDecoder   func([]byte) (*InboundMessage, error)
	MessageEncoder   func(*OutboundMessage) ([]byte, error)
	associatedValues map[string]any
}

func NewSocket(connection *websocket.Conn, messageDecoder func([]byte) (*InboundMessage, error), messageEncoder func(*OutboundMessage) ([]byte, error)) *Socket {
	return &Socket{
		connection:       connection,
		MessageDecoder:   messageDecoder,
		MessageEncoder:   messageEncoder,
		associatedValues: make(map[string]any),
	}
}

func (s *Socket) Close() error {
	return s.connection.Close(websocket.StatusNormalClosure, "")
}

func (s *Socket) Send(message *OutboundMessage) error {
	encodedMessage, err := s.MessageEncoder(message)
	if err != nil {
		return err
	}
	return s.connection.Write(context.Background(), websocket.MessageText, encodedMessage)
}

func (s *Socket) Set(key string, value any) {
	s.associatedValues[key] = value
}

func (s *Socket) Get(key string) any {
	return s.associatedValues[key]
}

func (s *Socket) handleNextMessageWithNode(node *HandlerNode) bool {
	_, msg, err := s.connection.Read(context.Background())
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
		return false
	}

	if err != nil {
		panic(err)
	}

	message, err := s.MessageDecoder(msg)
	if err != nil {
		panic(err)
	}
	if message.ID != "" {
		// TODO(rh): Maybe skip if the message has no ID?
	}

	ctx := NewContextWithNode(s, message, node)

	ctx.Next()
	ctx.free()

	return true
}
