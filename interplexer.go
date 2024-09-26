package scramjet

import (
	"context"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type Interplexer struct {
	mu sync.Mutex

	ID         string
	Connection InterplexerConnection

	LocalSockets         map[string]*Socket
	RemoteInterplexerIDs map[string]string
}

func NewInterplexer() *Interplexer {
	return &Interplexer{
		ID:                   uuid.NewString(),
		LocalSockets:         map[string]*Socket{},
		RemoteInterplexerIDs: map[string]string{},
	}
}

func (i *Interplexer) SetConnection(connection InterplexerConnection) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.Connection != nil {
		i.Connection.UnbindDispatch(i.ID)
		i.Connection.UnbindSocketOpenAnnounce()
		i.Connection.UnbindSocketCloseAnnounce()
		for socketID := range i.LocalSockets {
			i.Connection.AnnounceSocketClose(i.ID, socketID)
		}
	}

	if err := connection.BindDispatch(i.ID, i.handleDispatch); err != nil {
		return err
	}

	if err := connection.BindSocketOpenAnnounce(i.handleSocketOpenAnnounce); err != nil {
		return err
	}

	if err := connection.BindSocketCloseAnnounce(i.handleSocketCloseAnnounce); err != nil {
		return err
	}

	for socketID := range i.LocalSockets {
		if err := connection.AnnounceSocketOpen(i.ID, socketID); err != nil {
			return err
		}
	}

	i.Connection = connection

	return nil
}

func (i *Interplexer) AddLocalSocket(socket *Socket) {
	i.mu.Lock()
	i.LocalSockets[socket.id] = socket
	i.mu.Unlock()

	if i.Connection != nil {
		if err := i.Connection.AnnounceSocketOpen(i.ID, socket.id); err != nil {
			panic(err)
		}
	}
}

func (i *Interplexer) RemoveLocalSocket(socketID string) {
	i.mu.Lock()
	delete(i.LocalSockets, socketID)
	i.mu.Unlock()

	if i.Connection != nil {
		if err := i.Connection.AnnounceSocketClose(i.ID, socketID); err != nil {
			panic(err)
		}
	}
}

func (i *Interplexer) WithSocket(socketID string, messageDecoder func([]byte) (*InboundMessage, error), messageEncoder func(*OutboundMessage) ([]byte, error)) (*SocketHandle, bool) {
	i.mu.Lock()
	localSocket, hasLocalSocket := i.LocalSockets[socketID]
	interplexerID, hasRemoteSocket := i.RemoteInterplexerIDs[socketID]
	i.mu.Unlock()

	if !hasLocalSocket && !hasRemoteSocket {
		return nil, false
	}

	if hasLocalSocket {
		return &SocketHandle{
			kind:        SocketHandleKindLocal,
			localSocket: localSocket,
		}, true
	}

	return &SocketHandle{
		id:                  uuid.NewString(),
		kind:                SocketHandleKindRemote,
		remoteSocketID:      socketID,
		remoteInterplexerID: interplexerID,
		localInterplexer:    i,
		messageDecoder:      messageDecoder,
		messageEncoder:      messageEncoder,
	}, true
}

func (i *Interplexer) handleDispatch(socketID string, message []byte) bool {
	i.mu.Lock()
	localSocket, ok := i.LocalSockets[socketID]
	i.mu.Unlock()

	if !ok {
		return false
	}

	if err := localSocket.connection.Write(context.Background(), websocket.MessageBinary, message); err != nil {
		panic(err)
	}

	return true
}

func (i *Interplexer) handleSocketOpenAnnounce(interplexerID string, socketID string) {
	i.mu.Lock()
	i.RemoteInterplexerIDs[socketID] = interplexerID
	i.mu.Unlock()
}

func (i *Interplexer) handleSocketCloseAnnounce(interplexerID string, socketID string) {
	i.mu.Lock()
	delete(i.RemoteInterplexerIDs, socketID)
	i.mu.Unlock()
}
