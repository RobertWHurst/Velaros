package natsconnection

import (
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

type InterceptMessage struct {
	SocketID    string        `json:"socketId"`
	MessageID   string        `json:"messageId"`
	Timeout     time.Duration `json:"timeout"`
}

func (c *Connection) Intercept(interplexerID string, socketID string, messageID string, timeout time.Duration) error {
	messageBytes, err := json.Marshal(&InterceptMessage{
		SocketID:    socketID,
		MessageID:   messageID,
		Timeout:     timeout,
	})
	if err != nil {
		return err
	}
	return c.NatsConnection.Publish(namespace("socket.intercept", interplexerID), messageBytes)
}

func (c *Connection) BindIntercept(interplexerID string, handler func(socketID string, messageID string, timeout time.Duration)) error {
	sub, err := c.NatsConnection.Subscribe(namespace("socket.intercept", interplexerID), func(msg *nats.Msg) {
		interceptMessage := &InterceptMessage{}
		if err := json.Unmarshal(msg.Data, interceptMessage); err != nil {
			panic(err)
		}
		handler(interceptMessage.SocketID, interceptMessage.MessageID, interceptMessage.Timeout)
	})
	if err != nil {
		return err
	}

	c.unbindIntercept = func() error {
		return sub.Unsubscribe()
	}

	return nil
}

func (c *Connection) UnbindIntercept(interplexerID string) error {
	return c.unbindIntercept()
}