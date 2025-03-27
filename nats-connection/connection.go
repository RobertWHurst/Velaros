package natsconnection

import (
	"github.com/RobertWHurst/velaros"
	"github.com/nats-io/nats.go"
)

type Connection struct {
	NatsConnection            *nats.Conn
	unbindDispatch            func() error
	unbindSocketOpenAnnounce  func() error
	unbindSocketCloseAnnounce func() error
}

func New(conn *nats.Conn) velaros.InterplexerConnection {
	return &Connection{
		NatsConnection:            conn,
		unbindDispatch:            func() error { return nil },
		unbindSocketOpenAnnounce:  func() error { return nil },
		unbindSocketCloseAnnounce: func() error { return nil },
	}
}

type SocketIDs struct {
	InterplexerID string `json:"routerId"`
	SocketID      string `json:"socketId"`
}
