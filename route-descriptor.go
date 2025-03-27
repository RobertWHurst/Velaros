package velaros

import (
	"encoding/json"
)

type RouteDescriptor struct {
	Pattern *Pattern
}

// MarshalJSON returns the JSON representation of the route descriptor.
func (r *RouteDescriptor) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Pattern string
	}{
		Pattern: r.Pattern.String(),
	})
}

// UnmarshalJSON parses the JSON representation of the route descriptor.
func (r *RouteDescriptor) UnmarshalJSON(data []byte) error {
	fromJSONStruct := struct {
		Pattern string
	}{}
	if err := json.Unmarshal(data, &fromJSONStruct); err != nil {
		return err
	}

	pattern, err := NewPattern(fromJSONStruct.Pattern)
	if err != nil {
		return err
	}

	r.Pattern = pattern

	return nil
}
