package setvalue

import "github.com/RobertWHurst/velaros"

func Middleware[V any](key string, value *V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.Set(key, *value)
		ctx.Next()
	}
}
