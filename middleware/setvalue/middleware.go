package setvalue

import "github.com/RobertWHurst/velaros"

// Middleware creates middleware that sets a dereferenced pointer value on the
// message-level context. The pointer is dereferenced when the middleware is created,
// so updates to the pointed-to value after middleware creation won't be reflected.
// Values set at the message level only exist for the duration of that message's
// handler chain.
//
// Use this when you have a pointer to a value that you want to store by value.
//
// Example:
//
//	config := &AppConfig{MaxRetries: 3}
//	router.Use(setvalue.Middleware("config", config))  // Stores *config (dereferenced)
//
//	router.Bind("/process", func(ctx *velaros.Context) {
//	    cfg := ctx.MustGet("config").(AppConfig)  // Gets the value, not pointer
//	    // ...
//	})
//
// See also: set.Middleware for direct values, setfn.Middleware for dynamic values.
func Middleware[V any](key string, value *V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.Set(key, *value)
		ctx.Next()
	}
}
