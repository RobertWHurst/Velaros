package natsconnection

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type IgnoreMessage struct {
	SocketID  string `json:"socketId"`
	MessageID string `json:"messageId"`
}

func (c *Connection) Ignore(interplexerID string, socketID string, messageID string) error {
	messageBytes, err := json.Marshal(&IgnoreMessage{
		SocketID:  socketID,
		MessageID: messageID,
	})
	if err != nil {
		return err
	}
	return c.NatsConnection.Publish(namespace("socket.ignore", interplexerID), messageBytes)
}

func (c *Connection) BindIgnore(interplexerID string, handler func(socketID string, messageID string)) error {
	sub, err := c.NatsConnection.Subscribe(namespace("socket.ignore", interplexerID), func(msg *nats.Msg) {
		ignoreMessage := &IgnoreMessage{}
		if err := json.Unmarshal(msg.Data, ignoreMessage); err != nil {
			panic(err)
		}
		handler(ignoreMessage.SocketID, ignoreMessage.MessageID)
	})
	if err != nil {
		return err
	}

	c.unbindIgnore = func() error {
		return sub.Unsubscribe()
	}

	return nil
}

func (c *Connection) UnbindIgnore(interplexerID string) error {
	return c.unbindIgnore()
}