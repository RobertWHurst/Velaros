package scramjet

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type SocketHandleKind int

const (
	SocketHandleKindLocal SocketHandleKind = iota
	SocketHandleKindRemote
)

type SocketHandle struct {
	kind SocketHandleKind

	sourceSocketID string

	localSocket *socket

	remoteSocketID      string
	remoteInterplexerID string
	localInterplexer    *interplexer

	messageDecoder func([]byte) (*InboundMessage, error)
	messageEncoder func(*OutboundMessage) ([]byte, error)
}

func (h *SocketHandle) Send(data any) error {
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

func (h *SocketHandle) Request(data any) (any, error) {
	id := uuid.NewString()
	outboundMessage := &OutboundMessage{
		ID:   id,
		Data: data,
	}

	responseMessageChan := make(chan *InboundMessage, 1)
	sourceSocket := h.localInterplexer.localSockets[h.sourceSocketID]
	if sourceSocket == nil {
		return nil, errors.New("source socket not found")
	}
	sourceSocket.addInterceptor(id, func(message *InboundMessage) {
		responseMessageChan <- message
	})

	if h.kind == SocketHandleKindLocal {
		h.localSocket.send(outboundMessage)
	} else {
		messageData, err := h.messageEncoder(outboundMessage)
		if err != nil {
			return nil, err
		}
		if err := h.localInterplexer.connection.Dispatch(h.remoteInterplexerID, h.remoteSocketID, messageData); err != nil {
			return nil, err
		}
	}

	select {
	case message := <-responseMessageChan:
		return message.Data, nil
	case <-time.After(5 * time.Second):
		sourceSocket.removeInterceptor(id)
		return nil, errors.New("request timed out")
	}
}
