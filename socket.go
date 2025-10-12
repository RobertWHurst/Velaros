package velaros

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type socket struct {
	id                 string
	requestHeaders     http.Header
	connection         *websocket.Conn
	interceptorsMx     sync.Mutex
	interceptors       map[string]chan *InboundMessage
	associatedValuesMx sync.Mutex
	associatedValues   map[string]any
}

func newSocket(requestHeaders http.Header, conn *websocket.Conn) *socket {
	s := &socket{
		id:               uuid.NewString(),
		requestHeaders:   requestHeaders,
		connection:       conn,
		interceptors:     map[string]chan *InboundMessage{},
		associatedValues: map[string]any{},
	}
	return s
}

func (s *socket) close() error {
	return nil
}

func (s *socket) send(messageType websocket.MessageType, data []byte) error {
	return s.connection.Write(context.Background(), messageType, data)
}

func (s *socket) set(key string, value any) {
	s.associatedValuesMx.Lock()
	s.associatedValues[key] = value
	s.associatedValuesMx.Unlock()
}

func (s *socket) get(key string) (any, bool) {
	s.associatedValuesMx.Lock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.Unlock()
	return v, ok
}

func (s *socket) mustGet(key string) any {
	s.associatedValuesMx.Lock()
	v, ok := s.associatedValues[key]
	s.associatedValuesMx.Unlock()
	if !ok {
		panic(fmt.Sprintf("key %s not found", key))
	}
	return v
}

func (s *socket) handleNextMessageWithNode(node *HandlerNode) bool {
	msgType, msg, err := s.connection.Read(context.Background())
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway ||
		err == io.EOF {
		return false
	}
	if err != nil {
		panic(fmt.Errorf("error reading socket message: %w", err))
	}

	go func() {
		ctx := NewContextWithNodeAndMessageType(s, &InboundMessage{
			Data: msg,
		}, node, msgType)

		ctx.Next()
		ctx.free()
	}()

	return true
}

func (s *socket) getInterceptor(id string) (chan *InboundMessage, bool) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	interceptorChan, ok := s.interceptors[id]
	return interceptorChan, ok
}

func (s *socket) addInterceptor(id string, interceptorChan chan *InboundMessage) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	s.interceptors[id] = interceptorChan
}

func (s *socket) removeInterceptor(id string) {
	s.interceptorsMx.Lock()
	defer s.interceptorsMx.Unlock()

	delete(s.interceptors, id)
}
