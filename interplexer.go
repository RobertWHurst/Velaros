package scramjet

import (
	"sync"

	"github.com/google/uuid"
)

type Interplexer struct {
	mu sync.Mutex

	ID         string
	Connection InterplexerConnection

	LocalSockets         map[string]*Socket
	RemoteInterplexerIDs map[string]string
}

func NewInterplexer(connection InterplexerConnection) *Interplexer {
	i := &Interplexer{
		ID:         uuid.NewString(),
		Connection: connection,
	}

	if err := connection.BindDispatch(i.ID, func(socketID string, message []byte) bool {
		i.mu.Lock()
		localSocket, ok := i.LocalSockets[socketID]
		i.mu.Unlock()

		if !ok {
			return false
		}

		// TODO: Send message to local socket
		// TODO: Figure out how to do reply ids

		return true
	}); err != nil {
		panic(err)
	}

	if err := connection.BindSocketOpenAnnounce(func(interplexerID string, socketID string) {
		i.mu.Lock()
		i.RemoteInterplexerIDs[socketID] = interplexerID
		i.mu.Unlock()
	}); err != nil {
		panic(err)
	}

	if err := connection.BindSocketCloseAnnounce(func(interplexerID string, socketID string) {
		i.mu.Lock()
		delete(i.RemoteInterplexerIDs, socketID)
		i.mu.Unlock()
	}); err != nil {
		panic(err)
	}

	return i
}

func (i *Interplexer) AddLocalSocket(socket *Socket) {
	i.mu.Lock()
	i.LocalSockets[socket.id] = socket
	i.mu.Unlock()
}

func (i *Interplexer) RemoveLocalSocket(socketID string) {
	i.mu.Lock()
	delete(i.LocalSockets, socketID)
	i.mu.Unlock()
}

func (i *Interplexer) WithSocket(socketID string) (*SocketHandle, bool) {
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
	}, true
}
