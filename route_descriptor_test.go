package velaros_test

import (
	"encoding/json"
	"testing"

	"github.com/RobertWHurst/velaros"
)

func TestRouteDescriptorMarshalJSON(t *testing.T) {
	pattern, err := velaros.NewPattern("/a/b/c")
	if err != nil {
		t.Errorf("Failed to create pattern: %s", err.Error())
	}

	r := &velaros.RouteDescriptor{
		Pattern: pattern,
	}

	bytes, err := r.MarshalJSON()
	if err != nil {
		t.Errorf("Failed to marshal route descriptor: %s", err.Error())
	}

	jsonData := map[string]any{}
	err = json.Unmarshal(bytes, &jsonData)
	if err != nil {
		t.Errorf("Failed to unmarshal route descriptor: %s", err.Error())
	}

	if len(jsonData) != 1 {
		t.Errorf("Expected 1 key, got %d", len(jsonData))
	}
	if jsonData["Pattern"] != "/a/b/c" {
		t.Errorf("Expected Pattern to be /a/b/c, got %s", jsonData["Pattern"])
	}
}

func TestRouteDescriptorUnmarshalJSON(t *testing.T) {
	jsonData := []byte(`{"Pattern":"/a/b/c"}`)

	r := &velaros.RouteDescriptor{}
	if err := r.UnmarshalJSON(jsonData); err != nil {
		t.Errorf("Failed to unmarshal route descriptor: %s", err.Error())
	}

	if r.Pattern.String() != "/a/b/c" {
		t.Errorf("Expected Pattern to be /a/b/c, got %s", r.Pattern)
	}
}
