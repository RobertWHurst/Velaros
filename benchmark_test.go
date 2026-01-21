package velaros_test

import (
	"net/http/httptest"
	"testing"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
	"github.com/coder/websocket"
)

func BenchmarkPatternMatch(b *testing.B) {
	pattern, _ := velaros.NewPattern("/users/:userId/posts/:postId")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pattern.Match("/users/123/posts/456")
	}
}

func BenchmarkPatternMatchInto(b *testing.B) {
	pattern, _ := velaros.NewPattern("/users/:userId/posts/:postId")
	params := make(velaros.MessageParams)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pattern.MatchInto("/users/123/posts/456", &params)
	}
}

func BenchmarkPatternMatchStatic(b *testing.B) {
	pattern, _ := velaros.NewPattern("/api/users")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pattern.Match("/api/users")
	}
}

func BenchmarkPatternMatchWildcard(b *testing.B) {
	pattern, _ := velaros.NewPattern("/files/*")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pattern.Match("/files/some/deep/path/to/file.txt")
	}
}

func BenchmarkContextGetSet(b *testing.B) {
	router, server := setupRouter()
	defer server.Close()

	resultChan := make(chan struct{})

	router.Bind("/bench", func(ctx *velaros.Context) {
		for i := 0; i < b.N; i++ {
			ctx.Set("key", "value")
			_, _ = ctx.Get("key")
		}
		resultChan <- struct{}{}
	})

	conn, ctx := dialWebSocket(b, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	b.ResetTimer()
	writeMessage(b, conn, ctx, "", "/bench", nil)
	<-resultChan
}

func BenchmarkContextSocketGetSet(b *testing.B) {
	router, server := setupRouter()
	defer server.Close()

	resultChan := make(chan struct{})

	router.Bind("/bench", func(ctx *velaros.Context) {
		for i := 0; i < b.N; i++ {
			ctx.SetOnSocket("key", "value")
			_, _ = ctx.GetFromSocket("key")
		}
		resultChan <- struct{}{}
	})

	conn, ctx := dialWebSocket(b, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	b.ResetTimer()
	writeMessage(b, conn, ctx, "", "/bench", nil)
	<-resultChan
}

func BenchmarkMessageRouting(b *testing.B) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/bench", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "pong"}); err != nil {
			b.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(b, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writeMessage(b, conn, ctx, "", "/bench", nil)
		_, _ = readMessage(b, conn, ctx)
	}
}

func BenchmarkMessageRoutingWithMiddleware(b *testing.B) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	router.Use(func(ctx *velaros.Context) {
		ctx.Set("middleware1", true)
		ctx.Next()
	})

	router.Use(func(ctx *velaros.Context) {
		ctx.Set("middleware2", true)
		ctx.Next()
	})

	router.Bind("/bench", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "pong"}); err != nil {
			b.Errorf("send failed: %v", err)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	conn, ctx := dialWebSocket(b, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writeMessage(b, conn, ctx, "", "/bench", nil)
		_, _ = readMessage(b, conn, ctx)
	}
}

func BenchmarkRequestReplyRoundtrip(b *testing.B) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/echo", func(ctx *velaros.Context) {
		var msg testMessage
		if err := ctx.ReceiveInto(&msg); err != nil {
			b.Errorf("unmarshal failed: %v", err)
			return
		}
		if err := ctx.Send(msg); err != nil {
			b.Errorf("send failed: %v", err)
		}
	})

	conn, ctx := dialWebSocket(b, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writeMessage(b, conn, ctx, "", "/echo", testMessage{Msg: "ping"})
		_, _ = readMessage(b, conn, ctx)
	}
}

func BenchmarkConcurrentConnections(b *testing.B) {
	router, server := setupRouter()
	defer server.Close()

	router.Bind("/bench", func(ctx *velaros.Context) {
		if err := ctx.Send(testMessage{Msg: "pong"}); err != nil {
			b.Errorf("send failed: %v", err)
		}
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, ctx := dialWebSocket(b, server.URL)
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		for pb.Next() {
			writeMessage(b, conn, ctx, "", "/bench", nil)
			_, _ = readMessage(b, conn, ctx)
		}
	})
}

func BenchmarkContextPooling(b *testing.B) {
	router, server := setupRouter()
	defer server.Close()

	resultChan := make(chan struct{}, b.N)

	router.Bind("/bench", func(ctx *velaros.Context) {
		resultChan <- struct{}{}
	})

	conn, ctx := dialWebSocket(b, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writeMessage(b, conn, ctx, "", "/bench", nil)
		<-resultChan
	}
}

func BenchmarkPatternCompilation(b *testing.B) {
	patterns := []string{
		"/users",
		"/users/:id",
		"/users/:userId/posts/:postId",
		"/files/*",
		"/api/:version?/users",
		"/files/:path+",
		"/users/:id([0-9]+)",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range patterns {
			_, _ = velaros.NewPattern(p)
		}
	}
}

func BenchmarkParamsExtraction(b *testing.B) {
	router, server := setupRouter()
	defer server.Close()

	resultChan := make(chan struct{})

	router.Bind("/users/:userId/posts/:postId", func(ctx *velaros.Context) {
		params := ctx.Params()
		_ = params.Get("userId")
		_ = params.Get("postId")
		resultChan <- struct{}{}
	})

	conn, ctx := dialWebSocket(b, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	b.ResetTimer()
	writeMessage(b, conn, ctx, "", "/users/123/posts/456", nil)
	<-resultChan
}
