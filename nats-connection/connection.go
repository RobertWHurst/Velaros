package natsconnection

import (
	"encoding/json"

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

func (c *Connection) AnnounceSocketOpen(interplexerID string, socketID string) error {
	messageBytes, err := json.Marshal(SocketIDs{
		InterplexerID: interplexerID,
		SocketID:      socketID,
	})
	if err != nil {
		return err
	}
	c.NatsConnection.Publish(namespace("socket.open"), messageBytes)
	return nil
}

func (c *Connection) BindSocketOpenAnnounce(handler func(interplexerID string, socketID string)) error {
	socketOpenAnnounceSub, err := c.NatsConnection.Subscribe(namespace("socket.open"), func(msg *nats.Msg) {
		socketIDs := &SocketIDs{}
		if err := json.Unmarshal(msg.Data, socketIDs); err != nil {
			panic(err)
		}
		handler(socketIDs.InterplexerID, socketIDs.SocketID)
	})
	if err != nil {
		return err
	}

	unbinders, ok := c.unbindSocketOpenAnnounce["socket.open"]
	if !ok {
		unbinders = []func(){}
	}
	unbinders = append(unbinders, func() {
		socketOpenAnnounceSub.Unsubscribe()
	})
	c.unbindSocketOpenAnnounce["socket.open"] = unbinders

	return nil
}

func (c *Connection) AnnounceSocketClose(interplexerID string, socketID string) error {
	messageBytes, err := json.Marshal(SocketIDs{
		InterplexerID: interplexerID,
		SocketID:      socketID,
	})
	if err != nil {
		return err
	}
	c.NatsConnection.Publish(namespace("socket.close"), messageBytes)
	return nil
}

func (c *Connection) BindSocketCloseAnnounce(handler func(interplexerID string, socketID string)) error {
	socketCloseAnnounceSub, err := c.NatsConnection.Subscribe(namespace("socket.close"), func(msg *nats.Msg) {
		socketIDs := &SocketIDs{}
		if err := json.Unmarshal(msg.Data, socketIDs); err != nil {
			panic(err)
		}
		handler(socketIDs.InterplexerID, socketIDs.SocketID)
	})
	if err != nil {
		return err
	}

	unbinders, ok := c.unbindSocketCloseAnnounce["socket.close"]
	if !ok {
		unbinders = []func(){}
	}
	unbinders = append(unbinders, func() {
		socketCloseAnnounceSub.Unsubscribe()
	})
	c.unbindSocketCloseAnnounce["socket.close"] = unbinders

	return nil
}

func (c *Connection) Dispatch(interplexerID string, socketID string, message []byte) error {
	return nil
}

func (c *Connection) BindDispatch(interplexerID string, handler func(socketID string, message []byte) bool) error {
	return nil
}
