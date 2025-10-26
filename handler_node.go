package velaros

// BindType indicates the type of handler binding (regular message handler,
// open lifecycle handler, or close lifecycle handler).
type BindType int

const (
	// NormalBindType indicates a regular message handler bound to a path pattern.
	NormalBindType BindType = iota

	// OpenBindType indicates a lifecycle handler that runs when a connection opens.
	OpenBindType

	// CloseBindType indicates a lifecycle handler that runs when a connection closes.
	CloseBindType
)

// HandlerNode represents a node in the internal handler chain. This is an
// internal implementation detail used by the router to organize handlers into
// a linked list for efficient traversal during message processing.
//
// Each node represents either:
//   - A message handler bound to a specific path pattern (BindTypeBind)
//   - A connection open lifecycle handler (BindTypeBindOpen)
//   - A connection close lifecycle handler (BindTypeBindClose)
//
// Nodes are created automatically by the router when handlers are registered
// via Bind(), Use(), UseOpen(), or UseClose().
type HandlerNode struct {
	// BindType indicates whether this is a regular message handler, an open
	// lifecycle handler, or a close lifecycle handler.
	BindType BindType

	// Pattern is the route pattern that messages must match for this handler
	// to execute. Nil for lifecycle handlers (UseOpen/UseClose) and middleware
	// that runs on all paths.
	Pattern *Pattern

	// Handlers is the list of handler functions to execute when this node matches.
	// Multiple handlers can be attached to the same node and will execute in sequence.
	Handlers []any

	// Next is a pointer to the next handler node in the chain. Forms a linked list
	// that the router traverses to find matching handlers.
	Next *HandlerNode
}

// tryMatch attempts to match the current message path against this node's pattern.
// If the pattern matches, it extracts parameters and stores them in the context.
// Returns true if the pattern matches (or if there's no pattern), false otherwise.
//
// This is an internal method used by the routing logic.
func (n *HandlerNode) tryMatch(ctx *Context) bool {
	if n.Pattern == nil {
		return true
	}
	return n.Pattern.MatchInto(ctx.Path(), &ctx.params)
}
