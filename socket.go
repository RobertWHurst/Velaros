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
	forwarder        Forwarder
	interceptorsMx   sync.Mutex
	interceptors     map[string]func(*InboundMessage, []byte)
	messageDecoder   func([]byte) (*InboundMessage, error)
	messageEncoder   func(*OutboundMessage) ([]byte, error)
	associatedValues map[string]any
}

func NewSocket(conn SocketConnection, interplexer *interplexer, forwarder Forwarder, messageDecoder MessageDecoder, messageEncoder MessageEncoder) *Socket {
	s := &Socket{
		id:               uuid.NewString(),
		connection:       conn,
		interplexer:      interplexer,
		forwarder:        forwarder,
		interceptors:     map[string]func(*InboundMessage, []byte){},
		messageDecoder:   messageDecoder,
		messageEncoder:   messageEncoder,
		associatedValues: map[string]any{},
	}
	if interplexer != nil {
		interplexer.addLocalSocket(s)
	}
	return s
}

func (s *Socket) close() error {
	if s.interplexer != nil {
		s.interplexer.removeLocalSocket(s.id)
	}
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
	if s.forwarder != nil {
		s.forwarder.SetOnSocket(s.id, key, value)
		return
	}
	s.associatedValues[key] = value
}

func (s *Socket) get(key string) any {
	if s.forwarder != nil {
		return s.forwarder.GetFromSocket(s.id, key)
	}
	return s.associatedValues[key]
}

func (s *Socket) withSocket(socketID string) (SocketHandle, bool) {
	if socketID == s.id {
		panic("Cannot create a socket handle for the current socket. Try using send instead.")
	}
	if s.forwarder != nil {
		return s.forwarder.WithSocket(socketID, s.messageDecoder, s.messageEncoder)
	}
	if s.interplexer == nil {
		panic("Cannot use withSocket without interplexer")
	}
	return s.interplexer.withSocket(socketID, s.messageDecoder, s.messageEncoder)
}

func (s *Socket) handleNextMessageWithNode(node *HandlerNode) bool {
	messageData, err := s.connection.Read(context.Background())
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway ||
		err == io.EOF {
		return false
	}
	if err != nil {
		panic(fmt.Errorf("error reading socket message: %w", err))
	}

	go func() {
		message, err := s.messageDecoder(messageData)
		if err != nil {
			panic(err)
		}

		if s.maybeIntercept(message.ID, message, messageData) {
			return
		}

		ctx := NewContextWithNode(s, message, node)

		ctx.Next()
		ctx.free()
	}()

	return true
}

func (s *Socket) maybeIntercept(messageId string, message *InboundMessage, messageData []byte) bool {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	if interceptor, ok := s.interceptors[messageId]; ok {
		interceptor(message, messageData)
		delete(s.interceptors, messageId)
		return true
	}

	return false
}

func (s *Socket) addInterceptor(id string, interceptor func(*InboundMessage, []byte)) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	s.interceptors[id] = interceptor
}

func (s *Socket) removeInterceptor(id string) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	delete(s.interceptors, id)
}
