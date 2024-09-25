package scramjet_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/RobertWHurst/scramjet"
	"github.com/coder/websocket"
	"github.com/google/uuid"
)

func TestRouterSimpleHandler(t *testing.T) {
	router := scramjet.NewRouter()
	server := httptest.NewServer(router)

	type RequestData struct {
		Msg string `json:"msg"`
	}
	type ResponseData struct {
		Msg string `json:"msg"`
	}
	reqData := RequestData{
		Msg: "Hello Server",
	}
	resData := ResponseData{
		Msg: "Hello World",
	}

	router.Bind("/a/b/c", func(ctx *scramjet.Context) {
		ctx.UnmarshalMessageData(&reqData)
		ctx.Send(resData)
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

	type ReplyData struct {
		Msg string `json:"msg"`
	}
	type Reply struct {
		ID   string    `json:"id"`
		Data ReplyData `json:"data"`
	}

	reply := Reply{Data: ReplyData{}}
	err = json.Unmarshal(replyMsg, &reply)
	if err != nil {
		t.Fatal(err)
	}

	if reqData.Msg != "Hello Server" {
		t.Errorf("expected Hello Server, got %s", reqData.Msg)
	}

	if reply.ID == "" {
		t.Errorf("expected id to be set")
	}

	if err := uuid.Validate(reply.ID); err != nil {
		t.Errorf("expected valid uuid")
	}

	if reply.Data.Msg != "Hello World" {
		t.Errorf("expected Hello World, got %s", reply.Data.Msg)
	}
}
