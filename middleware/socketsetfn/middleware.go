package socketsetfn

import "github.com/RobertWHurst/velaros"

// Middleware creates middleware that sets a dynamically-generated value on the
// socket/connection-level context. The valueFn function is called once per connection
// when the first message is processed. The generated value persists for the lifetime
// of the WebSocket connection and is thread-safe.
//
// Use this when you need a unique value per connection (e.g., connection IDs, timestamps).
//
// Example:
//
//	router.Use(socketsetfn.Middleware("connID", func() string {
//	    return uuid.NewString()
//	}))
//
//	router.Bind("/data", func(ctx *velaros.Context) {
//	    connID := ctx.MustGetFromSocket("connID").(string)  // Same for all messages on this connection
//	    log.Printf("[%s] Processing message", connID)
//	})
//
// Note: For connection-specific setup, consider using router.UseOpen() instead.
//
// See also: socketset.Middleware for constant values, socketsetvalue.Middleware for pointer values.
func Middleware[V any](key string, valueFn func() V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.SetOnSocket(key, valueFn())
		ctx.Next()
	}
}
