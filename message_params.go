package velaros

import "strings"

// MessageParams represents the parameters extracted from the message path.
// Parameters are extracted from the path by matching the message path to
// the route pattern for the handler node. These may change each time Next
// is called on the context.
type MessageParams map[string]string

// Get returns the value of a parameter by key. The lookup is case-insensitive
// (e.g., 'ID' and 'id' match the same parameter). Returns an empty string if the
// key doesn't exist.
func (p MessageParams) Get(key string) string {
	for k, v := range p {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}
