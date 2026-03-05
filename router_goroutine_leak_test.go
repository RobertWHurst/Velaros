package velaros_test

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/RobertWHurst/velaros"
	jsonMiddleware "github.com/RobertWHurst/velaros/middleware/json"
)

// cancelableConnection wraps mockConnection but can be externally
// triggered to return context.Canceled, simulating the Eurus close path.
type cancelableConnection struct {
	incomingMessages chan *velaros.SocketMessage
	outgoingMessages chan *velaros.SocketMessage
	closed           bool
	mu               sync.Mutex
	cancelCh         chan struct{}
}

func newCancelableConnection() *cancelableConnection {
	return &cancelableConnection{
		incomingMessages: make(chan *velaros.SocketMessage, 10),
		outgoingMessages: make(chan *velaros.SocketMessage, 10),
		cancelCh:         make(chan struct{}),
	}
}

func (c *cancelableConnection) Read(ctx context.Context) (*velaros.SocketMessage, error) {
	select {
	case msg := <-c.incomingMessages:
		return msg, nil
	case <-c.cancelCh:
		return nil, context.Canceled
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *cancelableConnection) Write(ctx context.Context, msg *velaros.SocketMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return context.Canceled
	}
	select {
	case c.outgoingMessages <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *cancelableConnection) Close(status velaros.Status, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *cancelableConnection) sendIncoming(msg *velaros.SocketMessage) {
	c.incomingMessages <- msg
}

// TestHandlerGoroutineCleanupOnContextCanceled reproduces the goroutine leak
// that occurs when a connection closes via context.Canceled (the Eurus close
// path). Before the fix, handler goroutines blocked in ReceiveInto would never
// unblock because socket.cancelCtx() was never called.
func TestHandlerGoroutineCleanupOnContextCanceled(t *testing.T) {
	router := velaros.NewRouter()
	router.Use(jsonMiddleware.Middleware())

	handlerExited := make(chan struct{})

	router.Bind("/subscribe", func(ctx *velaros.Context) {
		defer close(handlerExited)
		// Block waiting for a follow-up message, like SubscribeEntry does.
		var msg map[string]any
		_ = ctx.ReceiveInto(&msg)
	})

	conn := newCancelableConnection()

	startGoroutines := runtime.NumGoroutine()

	// Start connection handler
	done := make(chan struct{})
	go func() {
		router.HandleConnection(nil, conn)
		close(done)
	}()

	// Build a subscribe message with an id (needed for ReceiveInto to work
	// on subsequent messages via the interceptor channel).
	subscribeMsg, _ := json.Marshal(map[string]any{
		"id":   "sub-1",
		"path": "/subscribe",
		"data": map[string]string{},
	})

	// Send a message to trigger the /subscribe handler
	conn.sendIncoming(&velaros.SocketMessage{
		Type:    velaros.MessageText,
		RawData: subscribeMsg,
	})

	// Give handler goroutine time to start and block on ReceiveInto
	time.Sleep(50 * time.Millisecond)

	// Simulate Eurus connection close (context.Canceled on next Read)
	close(conn.cancelCh)

	// Assert HandleConnection exits
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleConnection did not exit after context.Canceled")
	}

	// Assert handler goroutine exits (socket context was cancelled)
	select {
	case <-handlerExited:
	case <-time.After(2 * time.Second):
		t.Fatal("handler goroutine leaked: ReceiveInto did not unblock")
	}

	// Assert no goroutine leak
	time.Sleep(100 * time.Millisecond)
	endGoroutines := runtime.NumGoroutine()
	if endGoroutines > startGoroutines+2 {
		t.Errorf("goroutine leak: started=%d ended=%d", startGoroutines, endGoroutines)
	}
}
