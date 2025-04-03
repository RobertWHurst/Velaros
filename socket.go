package velaros

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type Socket struct {
	id               string
	connection       SocketConnection
	interplexer      *interplexer
	interceptorsMx   sync.Mutex
	interceptors     map[string]func(*InboundMessage)
	messageDecoder   func([]byte) (*InboundMessage, error)
	messageEncoder   func(*OutboundMessage) ([]byte, error)
	associatedValues map[string]any
}

func NewSocket(conn SocketConnection, interplexer *interplexer, messageDecoder func([]byte) (*InboundMessage, error), messageEncoder func(*OutboundMessage) ([]byte, error)) *Socket {
	s := &Socket{
		id:               uuid.NewString(),
		connection:       conn,
		interplexer:      interplexer,
		interceptors:     map[string]func(*InboundMessage){},
		messageDecoder:   messageDecoder,
		messageEncoder:   messageEncoder,
		associatedValues: map[string]any{},
	}
	interplexer.addLocalSocket(s)
	return s
}

func (s *Socket) close() error {
	s.interplexer.removeLocalSocket(s.id)
	return nil
}

func (s *Socket) send(message *OutboundMessage) error {
	encodedMessage, err := s.messageEncoder(message)
	if err != nil {
		return err
	}
	return s.connection.Write(context.Background(), encodedMessage)
}

func (s *Socket) set(key string, value any) {
	s.associatedValues[key] = value
}

func (s *Socket) get(key string) any {
	return s.associatedValues[key]
}

func (s *Socket) withSocket(socketID string) (*SocketHandle, bool) {
	if socketID == s.id {
		panic("Cannot create a socket handle for the current socket. Try using send instead.")
	}
	return s.interplexer.withSocket(s.id, socketID, s.messageDecoder, s.messageEncoder)
}

func (s *Socket) handleNextMessageWithNode(node *HandlerNode) bool {
	msg, err := s.connection.Read(context.Background())
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway ||
		err == io.EOF {
		return false
	}
	if err != nil {
		panic(fmt.Errorf("error reading socket message: %w", err))
	}

	go func() {
		message, err := s.messageDecoder(msg)
		if err != nil {
			panic(err)
		}

		// TODO: Move to a method and use defer to unlock
		s.interceptorsMx.Lock()
		if interceptor, ok := s.interceptors[message.ID]; ok {
			interceptor(message)
			delete(s.interceptors, message.ID)
			return
		}
		s.interceptorsMx.Unlock()

		ctx := NewContextWithNode(s, message, node)

		ctx.Next()
		ctx.free()
	}()

	return true
}

func (s *Socket) addInterceptor(id string, interceptor func(*InboundMessage)) {
	s.interceptors[id] = interceptor
}

func (s *Socket) removeInterceptor(id string) {
	delete(s.interceptors, id)
}
