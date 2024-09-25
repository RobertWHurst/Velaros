package natsconnection

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

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
