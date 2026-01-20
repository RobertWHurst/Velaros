package socketset

import "github.com/RobertWHurst/velaros"

// Middleware creates middleware that sets a value on the socket/connection-level context.
// The value is set once when the middleware is created and reused for all messages on
// all connections. Values set at the socket level persist for the lifetime of the WebSocket
// connection and are thread-safe (can be accessed from concurrent message handlers).
//
// Use this when you want to share a constant value across all handlers for a connection.
//
// Example:
//
//	router.Use(socketset.Middleware("serverVersion", "1.0.0"))
//
//	router.Bind("/info", func(ctx *velaros.Context) {
//	    version := ctx.MustGetFromSocket("serverVersion").(string)  // "1.0.0"
//	    ctx.Send(map[string]string{"version": version})
//	})
//
// Note: For connection-specific values like user sessions, use UseOpen instead.
//
// See also: socketsetfn.Middleware for dynamic values, socketsetvalue.Middleware for pointer values.
func Middleware[V any](key string, value V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.SetOnSocket(key, value)
		ctx.Next()
	}
}
