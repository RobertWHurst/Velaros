package natsconnection

import (
	"github.com/RobertWHurst/scramjet"
	"github.com/nats-io/nats.go"
)

type Connection struct {
	NatsConnection            *nats.Conn
	unbindDispatch            map[string][]func()
	unbindSocketOpenAnnounce  map[string][]func()
	unbindSocketCloseAnnounce map[string][]func()
}

func NewConnection(conn *nats.Conn) scramjet.InterplexerConnection {
	return &Connection{
		NatsConnection:            conn,
		unbindDispatch:            map[string][]func(){},
		unbindSocketOpenAnnounce:  map[string][]func(){},
		unbindSocketCloseAnnounce: map[string][]func(){},
	}
}

type SocketIDs struct {
	InterplexerID string `json:"routerId"`
	SocketID      string `json:"socketId"`
}
