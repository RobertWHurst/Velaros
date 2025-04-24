package velaros_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RobertWHurst/velaros"
	localconnection "github.com/RobertWHurst/velaros/local-connection"
	"github.com/coder/websocket"
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
	router := velaros.NewRouter()
	server := httptest.NewServer(router)

	reqData := RequestData{
		Msg: "Hello Server",
	}
	resData := ResponseData{
		Msg: "Hello World",
	}

	router.Bind("/a/b/c", func(ctx *velaros.Context) {
		if data := ctx.Data().(string); data != "Hello Server" {
			t.Fatalf("Expected message data to be Hello Server, got %s", data)
		}
		if err := ctx.Send(resData); err != nil {
			t.Fatal(err)
		}
	})

	ctx := context.Background()
	clientConn, _, err := websocket.Dial(ctx, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientConn.Close(websocket.StatusNormalClosure, "")

	msg := []byte(`{ "path": "/a/b/c", "data": "Hello Server" }`)
	if err = clientConn.Write(ctx, websocket.MessageText, msg); err != nil {
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
	router := velaros.NewRouter()
	server := httptest.NewServer(router)

	router.Use(func(ctx *velaros.Context) {
		ctx.Set("message", "Hello World")
		ctx.Next()
	})

	router.Bind("/a/b/c", func(ctx *velaros.Context) {
		if err := ctx.Send(ResponseData{
			Msg: ctx.Get("message").(string),
		}); err != nil {
			t.Fatalf("failed to send response: %v", err)
		}
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

	if reply.Data.Msg != "Hello World" {
		t.Errorf("expected Hello World, got %s", reply.Data.Msg)
	}
}

func TestRouterInterplexerWithSend(t *testing.T) {
	router1 := velaros.NewRouter()
	router2 := velaros.NewRouter()

	localConnection := localconnection.New()

	if err := router1.ConnectInterplexer(localConnection); err != nil {
		t.Fatalf("failed to connect interplexer: %v", err)
	}
	if err := router2.ConnectInterplexer(localConnection); err != nil {
		t.Fatalf("failed to connect interplexer: %v", err)
	}

	server1 := httptest.NewServer(router1)
	server2 := httptest.NewServer(router2)

	var socketID string
	router1.Bind("/attach", func(ctx *velaros.Context) {
		socketID = ctx.SocketID()
	})

	router2.Bind("/handle-message", func(ctx *velaros.Context) {
		if socketID == "" {
			t.Fatal("expected socketID to be set")
		}
		socket, ok := ctx.WithSocket(socketID)
		if !ok {
			t.Fatal("expected to find socket")
		}

		if err := socket.Send(ResponseData{
			Msg: "Hello World",
		}); err != nil {
			t.Fatalf("failed to send response: %v", err)
		}
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

func TestRouterInterplexerWithRequest(t *testing.T) {
	router1 := velaros.NewRouter()
	router2 := velaros.NewRouter()

	localConnection := localconnection.New()

	if err := router1.ConnectInterplexer(localConnection); err != nil {
		t.Fatalf("failed to connect interplexer: %v", err)
	}
	if err := router2.ConnectInterplexer(localConnection); err != nil {
		t.Fatalf("failed to connect interplexer: %v", err)
	}

	server1 := httptest.NewServer(router1)
	server2 := httptest.NewServer(router2)

	var socketID string
	router1.Bind("/attach", func(ctx *velaros.Context) {
		socketID = ctx.SocketID()
	})

	router2.Bind("/handle-message", func(ctx *velaros.Context) {
		if socketID == "" {
			t.Fatal("expected socketID to be set")
		}
		socket, ok := ctx.WithSocket(socketID)
		if !ok {
			t.Fatal("expected to find socket")
		}

		resMsg, err := socket.Request(ResponseData{
			Msg: "Server Request",
		})
		if err != nil {
			t.Fatalf("failed to send response: %v", err)
		}

		if resMsg.(string) != "Client Response" {
			t.Fatalf("expected Client Response, got %s", resMsg)
		}

		ctx.Reply("Server Response")
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

	if reply.Data.Msg != "Server Request" {
		t.Errorf("expected Hello World, got %s", reply.Data.Msg)
	}

	if err := clientConn1.Write(ctx, websocket.MessageText, []byte(`{ "id": "`+reply.ID+`", "data": "Client Response" }`)); err != nil {
		t.Fatal(err)
	}

	_, replyMsg, err = clientConn2.Read(ctx)
}
