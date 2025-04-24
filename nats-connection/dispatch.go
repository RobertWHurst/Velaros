package natsconnection

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type DispatchMessage struct {
	SocketID string `json:"socketId"`
	Message  []byte `json:"message"`
}

func (c *Connection) Dispatch(interplexerID string, socketID string, message []byte) error {
	messageBytes, err := json.Marshal(&DispatchMessage{
		SocketID: socketID,
		Message:  message,
	})
	if err != nil {
		return err
	}
	return c.NatsConnection.Publish(namespace("socket.dispatch", interplexerID), messageBytes)
}

func (c *Connection) BindDispatch(interplexerID string, handler func(socketID string, message []byte)) error {
	sub, err := c.NatsConnection.Subscribe(namespace("socket.dispatch", interplexerID), func(msg *nats.Msg) {
		dispatchMessage := &DispatchMessage{}
		if err := json.Unmarshal(msg.Data, dispatchMessage); err != nil {
			panic(err)
		}
		handler(dispatchMessage.SocketID, dispatchMessage.Message)
	})
	if err != nil {
		return err
	}

	c.unbindDispatch = func() error {
		return sub.Unsubscribe()
	}

	return nil
}

func (c *Connection) UnbindDispatch(interplexerID string) error {
	return c.unbindDispatch()
}
