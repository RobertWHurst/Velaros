package socketsetfn

import "github.com/RobertWHurst/velaros"

func Middleware[V any](key string, valueFn func() V) func(ctx *velaros.Context) {
	return func(ctx *velaros.Context) {
		ctx.SetOnSocket(key, valueFn())
		ctx.Next()
	}
}
