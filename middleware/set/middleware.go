package set

import "github.com/RobertWHurst/velaros"

// Middleware creates middleware that sets a value on the message-level context.
// The value is set once when the middleware is created and reused for all messages.
// Values set at the message level only exist for the duration of that message's
// handler chain.
//
// Use this when you want to share a constant value across handlers for each message.
//
// Example:
//
//	router.Use(set.Middleware("apiVersion", "v1"))
//
//	router.Bind("/info", func(ctx *velaros.Context) {
//	    version := ctx.MustGet("apiVersion").(string)  // "v1"
//	    ctx.Send(map[string]string{"version": version})
//	})
//
// See also: setfn.Middleware for dynamic values, setvalue.Middleware for pointer values.
func Middleware[V any](key string, value V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.Set(key, value)
		ctx.Next()
	}
}
