package natsconnection

import (
	"github.com/RobertWHurst/velaros"
	"github.com/nats-io/nats.go"
)

type Connection struct {
	NatsConnection            *nats.Conn
	unbindSocketOpenAnnounce  func() error
	unbindSocketCloseAnnounce func() error
	unbindDispatch            func() error
	unbindIntercept           func() error
	unbindIgnore              func() error
	unbindIntercepted         func() error
}

func New(conn *nats.Conn) velaros.InterplexerConnection {
	return &Connection{
		NatsConnection:            conn,
		unbindSocketOpenAnnounce:  func() error { return nil },
		unbindSocketCloseAnnounce: func() error { return nil },
		unbindDispatch:            func() error { return nil },
		unbindIntercept:           func() error { return nil },
		unbindIgnore:              func() error { return nil },
		unbindIntercepted:         func() error { return nil },
	}
}

type SocketIDs struct {
	InterplexerID string `json:"routerId"`
	SocketID      string `json:"socketId"`
}
