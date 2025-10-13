package socketsetvalue

import "github.com/RobertWHurst/velaros"

// Middleware creates middleware that sets a dereferenced pointer value on the
// socket/connection-level context. The pointer is dereferenced when the first message
// is processed, so updates to the pointed-to value after that won't be reflected.
// The value persists for the lifetime of the WebSocket connection and is thread-safe.
//
// Use this when you have a pointer to a value that you want to store by value at the
// connection level.
//
// Example:
//
//	config := &ServerConfig{Timeout: 30}
//	router.Use(socketsetvalue.Middleware("config", config))  // Stores *config (dereferenced)
//
//	router.Bind("/process", func(ctx *velaros.Context) {
//	    cfg := ctx.MustGetFromSocket("config").(ServerConfig)  // Gets the value, not pointer
//	    // ...
//	})
//
// Note: The value is dereferenced on first message, not at middleware creation time.
//
// See also: socketset.Middleware for direct values, socketsetfn.Middleware for dynamic values.
func Middleware[V any](key string, value *V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.SetOnSocket(key, *value)
		ctx.Next()
	}
}
