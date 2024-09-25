package scramjet

type SocketHandleKind int

const (
	SocketHandleKindLocal SocketHandleKind = iota
	SocketHandleKindRemote
)

type SocketHandle struct {
	id   string
	kind SocketHandleKind

	localSocket *Socket

	remoteSocketID      string
	remoteInterplexerID string
	localInterplexer    *Interplexer

	messageDecoder func([]byte) (*InboundMessage, error)
	messageEncoder func(*OutboundMessage) ([]byte, error)
}

func (h *SocketHandle) Send(data any) error {
	if h.kind == SocketHandleKindLocal {
		return h.localSocket.Send(&OutboundMessage{
			ID:   h.id,
			Data: data,
		})
	}

	messageData, err := h.messageEncoder(&OutboundMessage{
		ID:   h.id,
		Data: data,
	})
	if err != nil {
		return err
	}

	return h.localInterplexer.Connection.Dispatch(h.remoteInterplexerID, h.remoteSocketID, messageData)
}

// func (h *SocketHandle) Request(data any) (any, error) {
// 	if h.kind == SocketHandleKindLocal {
// 		return h.localSocket.Request(&OutboundMessage{
// 			ID:   h.id,
// 			Data: data,
// 		})
// 	}

// 	// TODO: Figure out how to do reply ids
// 	err := h.localInterplexer.Connection.Dispatch(h.remoteInterplexerID, h.remoteSocketID, data)
// }
