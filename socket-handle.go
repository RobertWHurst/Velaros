package velaros

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SocketHandle provides an interface for interacting with a socket
// through methods like Send and Request
type SocketHandle interface {
	// Send sends a message to the socket
	Send(data any) error

	// Request sends a message to the socket and waits for a response
	Request(data any) (any, error)
}

type SocketHandleKind int

const (
	SocketHandleKindLocal SocketHandleKind = iota
	SocketHandleKindRemote
)

// socketHandle is the internal implementation of the SocketHandle interface
type socketHandle struct {
	kind SocketHandleKind

	localSocket *Socket

	remoteSocketID      string
	remoteInterplexerID string
	localInterplexer    *interplexer

	messageDecoder func([]byte) (*InboundMessage, error)
	messageEncoder func(*OutboundMessage) ([]byte, error)
}

// Ensure socketHandle implements SocketHandle
var _ SocketHandle = &socketHandle{}

func (h *socketHandle) Send(data any) error {
	if h.kind == SocketHandleKindLocal {
		return h.localSocket.send(&OutboundMessage{
			Data: data,
		})
	}

	messageData, err := h.messageEncoder(&OutboundMessage{
		Data: data,
	})
	if err != nil {
		return err
	}

	return h.localInterplexer.connection.Dispatch(h.remoteInterplexerID, h.remoteSocketID, messageData)
}

func (h *socketHandle) RequestWithContext(ctx context.Context, data any) (any, error) {
	id := uuid.NewString()
	responseMessageChan := make(chan *InboundMessage, 1)

	if h.kind == SocketHandleKindLocal {
		h.localSocket.addInterceptor(id, func(message *InboundMessage, _ []byte) {
			responseMessageChan <- message
		})
		defer h.localSocket.removeInterceptor(id)

		if err := h.localSocket.send(&OutboundMessage{
			ID:   id,
			Data: data,
		}); err != nil {
			return nil, err
		}

	} else {
		deadline, ok := ctx.Deadline()
		timeout := DefaultRequestTimeout
		if ok {
			timeout = time.Until(deadline)
			if timeout < 1 {
				timeout = 1
			}
		}

		h.localInterplexer.addRemoteInterceptor(h.remoteInterplexerID, h.remoteSocketID, id, timeout, func(message *InboundMessage, _ []byte) {
			responseMessageChan <- message
		})
		defer h.localInterplexer.removeRemoteInterceptor(h.remoteInterplexerID, h.remoteSocketID, id)

		messageData, err := h.messageEncoder(&OutboundMessage{
			ID:   id,
			Data: data,
		})
		if err != nil {
			return nil, err
		}

		if err := h.localInterplexer.connection.Dispatch(h.remoteInterplexerID, h.remoteSocketID, messageData); err != nil {
			return nil, err
		}
	}

	select {
	case responseMessage := <-responseMessageChan:
		return responseMessage.Data, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
	}
}

func (h *socketHandle) RequestWithTimeout(data any, timeout time.Duration) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return h.RequestWithContext(ctx, data)
}

func (h *socketHandle) Request(data any) (any, error) {
	return h.RequestWithTimeout(data, DefaultRequestTimeout)
}
