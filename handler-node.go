package scramjet

type HandlerNode struct {
	pattern                *Pattern
	handlersOrTransformers []any
	nextNode               *HandlerNode
}
