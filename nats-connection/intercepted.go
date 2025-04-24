package natsconnection

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type InterceptedMessage struct {
	SocketID  string `json:"socketId"`
	MessageID string `json:"messageId"`
	Message   []byte `json:"message"`
}

func (c *Connection) Intercepted(interplexerID string, socketID string, messageID string, message []byte) error {
	messageBytes, err := json.Marshal(&InterceptedMessage{
		SocketID:  socketID,
		MessageID: messageID,
		Message:   message,
	})
	if err != nil {
		return err
	}
	return c.NatsConnection.Publish(namespace("socket.intercepted", interplexerID), messageBytes)
}

func (c *Connection) BindIntercepted(interplexerID string, handler func(socketID string, messageID string, message []byte)) error {
	sub, err := c.NatsConnection.Subscribe(namespace("socket.intercepted", interplexerID), func(msg *nats.Msg) {
		interceptedMessage := &InterceptedMessage{}
		if err := json.Unmarshal(msg.Data, interceptedMessage); err != nil {
			panic(err)
		}
		handler(interceptedMessage.SocketID, interceptedMessage.MessageID, interceptedMessage.Message)
	})
	if err != nil {
		return err
	}

	c.unbindIntercepted = func() error {
		return sub.Unsubscribe()
	}

	return nil
}

func (c *Connection) UnbindIntercepted(interplexerID string) error {
	return c.unbindIntercepted()
}