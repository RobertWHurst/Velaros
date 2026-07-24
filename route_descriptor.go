package velaros

import (
	"encoding/json"
)

// RouteDescriptor describes a public route registered with PublicBind. Route descriptors
// are used by API gateway frameworks for service discovery and routing. Access them via
// Router.RouteDescriptors().
type RouteDescriptor struct {
	Pattern *Pattern

	// Metadata carries arbitrary route metadata attached with WithMetadata.
	// Gateways and middleware can use it to implement features like rate
	// limiting or auth requirements. After a JSON round trip it holds a
	// json.RawMessage which consumers can decode into their own types.
	Metadata any
}

// MarshalJSON returns the JSON representation of the route descriptor.
func (r *RouteDescriptor) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Pattern  string
		Metadata any `json:",omitempty"`
	}{
		Pattern:  r.Pattern.String(),
		Metadata: r.Metadata,
	})
}

// UnmarshalJSON parses the JSON representation of the route descriptor.
func (r *RouteDescriptor) UnmarshalJSON(data []byte) error {
	fromJSONStruct := struct {
		Pattern  string
		Metadata json.RawMessage
	}{}
	if err := json.Unmarshal(data, &fromJSONStruct); err != nil {
		return err
	}

	pattern, err := NewPattern(fromJSONStruct.Pattern)
	if err != nil {
		return err
	}

	r.Pattern = pattern
	if len(fromJSONStruct.Metadata) > 0 {
		r.Metadata = fromJSONStruct.Metadata
	}

	return nil
}
