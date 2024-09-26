package natsconnection

import (
	"github.com/RobertWHurst/scramjet"
	"github.com/nats-io/nats.go"
)

type Connection struct {
	NatsConnection            *nats.Conn
	unbindDispatch            func()
	unbindSocketOpenAnnounce  func()
	unbindSocketCloseAnnounce func()
}

func New(conn *nats.Conn) scramjet.InterplexerConnection {
	return &Connection{
		NatsConnection:            conn,
		unbindDispatch:            func() {},
		unbindSocketOpenAnnounce:  func() {},
		unbindSocketCloseAnnounce: func() {},
	}
}

type SocketIDs struct {
	InterplexerID string `json:"routerId"`
	SocketID      string `json:"socketId"`
}
