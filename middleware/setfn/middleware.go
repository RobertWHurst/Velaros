package setfn

import "github.com/RobertWHurst/velaros"

// Middleware creates middleware that sets a dynamically-generated value on the
// message-level context. The valueFn function is called for each message to generate
// a fresh value. Values set at the message level only exist for the duration of that
// message's handler chain.
//
// Use this when you need a unique value for each message (e.g., timestamps, UUIDs, request IDs).
//
// Example:
//
//	router.Use(setfn.Middleware("requestID", func() string {
//	    return uuid.NewString()
//	}))
//
//	router.Bind("/data", func(ctx *velaros.Context) {
//	    requestID := ctx.MustGet("requestID").(string)  // Unique per message
//	    log.Printf("[%s] Processing request", requestID)
//	})
//
// See also: set.Middleware for constant values, setvalue.Middleware for pointer values.
func Middleware[V any](key string, valueFn func() V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.Set(key, valueFn())
		ctx.Next()
	}
}
