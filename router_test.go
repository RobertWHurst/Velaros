package scramjet_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/RobertWHurst/scramjet"
	"github.com/coder/websocket"
)

func TestRouterSimpleHandler(t *testing.T) {
	router := scramjet.NewRouter()
	server := httptest.NewServer(router)

	serverRes := struct {
		Msg string `json:"msg"`
	}{
		Msg: "Hello World",
	}

	router.Bind("/a/b/c", func(ctx *scramjet.Context) {
		ctx.Reply(serverRes)
	})

	ctx := context.Background()
	clientConn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close(websocket.StatusNormalClosure, "")

	msg := []byte(`{ "path": "/a/b/c", "msg": "Hello Server" }`)
	err = clientConn.Write(ctx, websocket.MessageText, msg)
	if err != nil {
		t.Fatal(err)
	}

	_, replyMsg, err := clientConn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if string(replyMsg) != "Hello World" {
		t.Errorf("expected Hello World, got %s", string(replyMsg))
	}
}
