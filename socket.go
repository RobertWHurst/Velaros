package scramjet

import (
	"context"

	"github.com/coder/websocket"
)

type Socket struct {
	connection       *websocket.Conn
	id               string
	messageDecoder   func([]byte) (*InboundMessage, error)
	messageEncoder   func(*OutboundMessage) ([]byte, error)
	associatedValues map[string]any
}

func NewSocket(conn *websocket.Conn, messageDecoder func([]byte) (*InboundMessage, error), messageEncoder func(*OutboundMessage) ([]byte, error)) *Socket {
	return &Socket{
		connection:       conn,
		messageDecoder:   messageDecoder,
		messageEncoder:   messageEncoder,
		associatedValues: make(map[string]any),
	}
}

func (s *Socket) Close() error {
	return s.connection.Close(websocket.StatusNormalClosure, "")
}

func (s *Socket) Send(message *OutboundMessage) error {
	encodedMessage, err := s.messageEncoder(message)
	if err != nil {
		return err
	}
	return s.connection.Write(context.Background(), websocket.MessageBinary, encodedMessage)
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

	message, err := s.messageDecoder(msg)
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
