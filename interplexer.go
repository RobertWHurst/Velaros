package velaros

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type interplexer struct {
	mu sync.Mutex

	id         string
	connection InterplexerConnection

	localSockets              map[string]*Socket
	remoteInterplexerIDs      map[string]string
	remoteInterceptorHandlers map[string]func(*InboundMessage, []byte)

	messageDecoder MessageDecoder
	messageEncoder MessageEncoder
}

func newInterplexer() *interplexer {
	return &interplexer{
		id:                        uuid.NewString(),
		localSockets:              map[string]*Socket{},
		remoteInterplexerIDs:      map[string]string{},
		remoteInterceptorHandlers: map[string]func(*InboundMessage, []byte){},

		messageDecoder: DefaultMessageDecoder,
		messageEncoder: DefaultMessageEncoder,
	}
}

func (i *interplexer) setConnection(connection InterplexerConnection) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.connection != nil {
		if err := i.connection.UnbindSocketOpenAnnounce(); err != nil {
			fmt.Printf("error unbinding previous interplexer socket open announce: %v\n", err)
		}
		if err := i.connection.UnbindSocketCloseAnnounce(); err != nil {
			fmt.Printf("error unbinding previous interplexer socket close announce: %v\n", err)
		}
		if err := i.connection.UnbindDispatch(i.id); err != nil {
			fmt.Printf("error unbinding previous interplexer dispatch: %v\n", err)
		}
		if err := i.connection.UnbindIntercept(i.id); err != nil {
			fmt.Printf("error unbinding previous interplexer intercept: %v\n", err)
		}
		if err := i.connection.UnbindIgnore(i.id); err != nil {
			fmt.Printf("error unbinding previous interplexer ignore: %v\n", err)
		}
		if err := i.connection.UnbindIntercepted(i.id); err != nil {
			fmt.Printf("error unbinding previous interplexer intercepted: %v\n", err)
		}
		for socketID := range i.localSockets {
			i.connection.AnnounceSocketClose(i.id, socketID)
		}
	}

	if err := connection.BindSocketOpenAnnounce(i.handleSocketOpenAnnounce); err != nil {
		return err
	}
	if err := connection.BindSocketCloseAnnounce(i.handleSocketCloseAnnounce); err != nil {
		return err
	}
	if err := connection.BindDispatch(i.id, i.handleDispatch); err != nil {
		return err
	}
	if err := connection.BindIntercept(i.id, i.handleIntercept); err != nil {
		return err
	}
	if err := connection.BindIgnore(i.id, i.handleIgnore); err != nil {
		return err
	}
	if err := connection.BindIntercepted(i.id, i.handleIntercepted); err != nil {
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

func (i *interplexer) addLocalSocket(socket *Socket) {
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

func (i *interplexer) addRemoteInterceptor(interplexerID string, socketID string, messageId string, timeout time.Duration, handler func(*InboundMessage, []byte)) {
	i.connection.Intercept(interplexerID, socketID, messageId, timeout)
	i.mu.Lock()
	i.remoteInterceptorHandlers[messageId] = handler
	i.mu.Unlock()
}

func (i *interplexer) removeRemoteInterceptor(interplexerID string, socketID string, messageID string) {
	i.mu.Lock()
	delete(i.remoteInterceptorHandlers, messageID)
	i.mu.Unlock()
	i.connection.Ignore(interplexerID, socketID, messageID)
}

func (i *interplexer) withSocket(socketID string, messageDecoder func([]byte) (*InboundMessage, error), messageEncoder func(*OutboundMessage) ([]byte, error)) (SocketHandle, bool) {
	i.mu.Lock()
	localSocket, hasLocalSocket := i.localSockets[socketID]
	interplexerID, hasRemoteSocket := i.remoteInterplexerIDs[socketID]
	i.mu.Unlock()

	if !hasLocalSocket && !hasRemoteSocket {
		return nil, false
	}

	if hasLocalSocket {
		return &socketHandle{
			kind:        SocketHandleKindLocal,
			localSocket: localSocket,
		}, true
	}

	return &socketHandle{
		kind:                SocketHandleKindRemote,
		remoteSocketID:      socketID,
		remoteInterplexerID: interplexerID,
		localInterplexer:    i,
		messageDecoder:      messageDecoder,
		messageEncoder:      messageEncoder,
	}, true
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

func (i *interplexer) handleDispatch(socketID string, messageData []byte) {
	i.mu.Lock()
	localSocket, ok := i.localSockets[socketID]
	i.mu.Unlock()

	if !ok {
		return
	}

	if err := localSocket.connection.Write(context.Background(), messageData); err != nil {
		panic(err)
	}
}

func (i *interplexer) handleIntercept(socketID string, messageId string, timeout time.Duration) {
	i.mu.Lock()
	localSocket, ok := i.localSockets[socketID]
	i.mu.Unlock()

	if !ok {
		return
	}

	localSocket.addInterceptor(messageId, func(_ *InboundMessage, messageData []byte) {
		if err := i.connection.Intercepted(i.id, localSocket.id, messageId, messageData); err != nil {
			panic(err)
		}
	})

	go func() {
		<-time.After(timeout)
		localSocket.removeInterceptor(messageId)
	}()
}

func (i *interplexer) handleIgnore(socketID string, messageId string) {
	i.mu.Lock()
	localSocket, ok := i.localSockets[socketID]
	i.mu.Unlock()

	if !ok {
		return
	}

	localSocket.removeInterceptor(messageId)
}

func (i *interplexer) handleIntercepted(socketID string, messageId string, messageData []byte) {
	i.mu.Lock()
	handler, ok := i.remoteInterceptorHandlers[messageId]
	i.mu.Unlock()

	if !ok {
		return
	}

	message, err := i.messageDecoder(messageData)
	if err != nil {
		panic(err)
	}
	handler(message, messageData)
}
