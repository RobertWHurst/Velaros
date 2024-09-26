package scramjet_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RobertWHurst/scramjet"
	localconnection "github.com/RobertWHurst/scramjet/local-connection"
	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type RequestData struct {
	Msg string `json:"msg"`
}

type ResponseData struct {
	Msg string `json:"msg"`
}

type ReplyData struct {
	Msg string `json:"msg"`
}

type Reply struct {
	ID   string    `json:"id"`
	Data ReplyData `json:"data"`
}

func TestRouterSimpleHandler(t *testing.T) {
	router := scramjet.NewRouter()
	server := httptest.NewServer(router)

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

	reply := Reply{Data: ReplyData{}}
	err = json.Unmarshal(replyMsg, &reply)
	if err != nil {
		t.Fatal(err)
	}

	if reqData.Msg != "Hello Server" {
		t.Errorf("expected Hello Server, got %s", reqData.Msg)
	}

	if reply.Data.Msg != "Hello World" {
		t.Errorf("expected Hello World, got %s", reply.Data.Msg)
	}
}

func TestRouterMiddleware(t *testing.T) {
	router := scramjet.NewRouter()
	server := httptest.NewServer(router)

	router.Use(func(ctx *scramjet.Context) {
		ctx.Set("message", "Hello World")
		ctx.Next()
	})

	router.Bind("/a/b/c", func(ctx *scramjet.Context) {
		ctx.Send(ResponseData{
			Msg: ctx.Get("message").(string),
		})
	})

	ctx := context.Background()
	clientConn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close(websocket.StatusNormalClosure, "")

	msg := []byte(`{ "path": "/a/b/c" }`)
	err = clientConn.Write(ctx, websocket.MessageText, msg)
	if err != nil {
		t.Fatal(err)
	}

	_, replyMsg, err := clientConn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	reply := Reply{Data: ReplyData{}}
	err = json.Unmarshal(replyMsg, &reply)
	if err != nil {
		t.Fatal(err)
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

func TestRouterInterplexer(t *testing.T) {
	router1 := scramjet.NewRouter()
	router2 := scramjet.NewRouter()

	localConnection := localconnection.New()

	router1.ConnectInterplexer(localConnection)
	router2.ConnectInterplexer(localConnection)

	server1 := httptest.NewServer(router1)
	server2 := httptest.NewServer(router2)

	var socketID string
	router1.Bind("/attach", func(ctx *scramjet.Context) {
		socketID = ctx.SocketID()
	})

	router2.Bind("/handle-message", func(ctx *scramjet.Context) {
		if socketID == "" {
			t.Fatal("expected socketID to be set")
		}
		socket, ok := ctx.WithSocket(socketID)
		if !ok {
			t.Fatal("expected to find socket")
		}

		socket.Send(ResponseData{
			Msg: "Hello World",
		})
	})

	ctx := context.Background()
	clientConn1, _, err := websocket.Dial(ctx, server1.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn1.Close(websocket.StatusNormalClosure, "")

	clientConn2, _, err := websocket.Dial(ctx, server2.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn2.Close(websocket.StatusNormalClosure, "")

	msg := []byte(`{ "path": "/attach" }`)
	err = clientConn1.Write(ctx, websocket.MessageText, msg)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	msg = []byte(`{ "path": "/handle-message" }`)
	err = clientConn2.Write(ctx, websocket.MessageText, msg)
	if err != nil {
		t.Fatal(err)
	}

	_, replyMsg, err := clientConn1.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	reply := Reply{Data: ReplyData{}}
	err = json.Unmarshal(replyMsg, &reply)
	if err != nil {
		t.Fatal(err)
	}

	if reply.Data.Msg != "Hello World" {
		t.Errorf("expected Hello World, got %s", reply.Data.Msg)
	}
}
