package scramjet

import (
	"context"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type interplexer struct {
	mu sync.Mutex

	id         string
	connection InterplexerConnection

	localSockets         map[string]*socket
	remoteInterplexerIDs map[string]string
}

func newInterplexer() *interplexer {
	return &interplexer{
		id:                   uuid.NewString(),
		localSockets:         map[string]*socket{},
		remoteInterplexerIDs: map[string]string{},
	}
}

func (i *interplexer) setConnection(connection InterplexerConnection) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.connection != nil {
		i.connection.UnbindDispatch(i.id)
		i.connection.UnbindSocketOpenAnnounce()
		i.connection.UnbindSocketCloseAnnounce()
		for socketID := range i.localSockets {
			i.connection.AnnounceSocketClose(i.id, socketID)
		}
	}

	if err := connection.BindDispatch(i.id, i.handleDispatch); err != nil {
		return err
	}

	if err := connection.BindSocketOpenAnnounce(i.handleSocketOpenAnnounce); err != nil {
		return err
	}

	if err := connection.BindSocketCloseAnnounce(i.handleSocketCloseAnnounce); err != nil {
		return err
	}

	for socketID := range i.localSockets {
		if err := connection.AnnounceSocketOpen(i.id, socketID); err != nil {
			return err
		}
	}

	i.connection = connection

	return nil
}

func (i *interplexer) addLocalSocket(socket *socket) {
	i.mu.Lock()
	i.localSockets[socket.id] = socket
	i.mu.Unlock()

	if i.connection != nil {
		if err := i.connection.AnnounceSocketOpen(i.id, socket.id); err != nil {
			panic(err)
		}
	}
}

func (i *interplexer) removeLocalSocket(socketID string) {
	i.mu.Lock()
	delete(i.localSockets, socketID)
	i.mu.Unlock()

	if i.connection != nil {
		if err := i.connection.AnnounceSocketClose(i.id, socketID); err != nil {
			panic(err)
		}
	}
}

func (i *interplexer) withSocket(sourceSocketID, socketID string, messageDecoder func([]byte) (*InboundMessage, error), messageEncoder func(*OutboundMessage) ([]byte, error)) (*SocketHandle, bool) {
	i.mu.Lock()
	localSocket, hasLocalSocket := i.localSockets[socketID]
	interplexerID, hasRemoteSocket := i.remoteInterplexerIDs[socketID]
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
		kind:                SocketHandleKindRemote,
		sourceSocketID:      sourceSocketID,
		remoteSocketID:      socketID,
		remoteInterplexerID: interplexerID,
		localInterplexer:    i,
		messageDecoder:      messageDecoder,
		messageEncoder:      messageEncoder,
	}, true
}

func (i *interplexer) handleDispatch(socketID string, message []byte) bool {
	i.mu.Lock()
	localSocket, ok := i.localSockets[socketID]
	i.mu.Unlock()

	if !ok {
		return false
	}

	if err := localSocket.connection.Write(context.Background(), websocket.MessageBinary, message); err != nil {
		panic(err)
	}

	return true
}

func (i *interplexer) handleSocketOpenAnnounce(interplexerID string, socketID string) {
	i.mu.Lock()
	i.remoteInterplexerIDs[socketID] = interplexerID
	i.mu.Unlock()
}

func (i *interplexer) handleSocketCloseAnnounce(interplexerID string, socketID string) {
	i.mu.Lock()
	delete(i.remoteInterplexerIDs, socketID)
	i.mu.Unlock()
}
