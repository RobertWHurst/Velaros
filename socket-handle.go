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
}

func (h *SocketHandle) Send(data any) error {
	if h.kind == SocketHandleKindLocal {
		return h.localSocket.Send(&OutboundMessage{
			ID:   h.id,
			Data: data,
		})
	}

	// TODO: Figure out how to deal with encoding data on the interplexer.
	// Using the message encoder or decoder might not be a good idea as it
	// could be different in different instances of router accross the interplexer
	// network. The problem is that it would require either the end user define
	// struct tags for a fixed interplexer encoder/decoder to use, or provide
	// a way to define a custom encoder/decoder for the interplexer.

	return h.localInterplexer.Connection.Dispatch(h.remoteInterplexerID, h.remoteSocketID, data)
}
