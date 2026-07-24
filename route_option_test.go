package velaros_test

import (
	"encoding/json"
	"testing"

	"github.com/RobertWHurst/velaros"
)

func TestWithMetadata_AttachesToRouteDescriptor(t *testing.T) {
	router := velaros.NewRouter()
	router.PublicBind("/api/items/:id", velaros.WithMetadata(map[string]any{
		"auth": "required",
	}), func(ctx *velaros.Context) {})

	descriptors := router.RouteDescriptors()
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 route descriptor, got %d", len(descriptors))
	}

	metadata, ok := descriptors[0].Metadata.(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map, got %T", descriptors[0].Metadata)
	}
	if metadata["auth"] != "required" {
		t.Errorf("expected auth=required, got %v", metadata["auth"])
	}
}

func TestWithMetadata_OmittedWhenNotProvided(t *testing.T) {
	router := velaros.NewRouter()
	router.PublicBind("/api/items", func(ctx *velaros.Context) {})

	descriptors := router.RouteDescriptors()
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 route descriptor, got %d", len(descriptors))
	}
	if descriptors[0].Metadata != nil {
		t.Errorf("expected nil metadata, got %v", descriptors[0].Metadata)
	}
}

func TestWithMetadata_IsNotTreatedAsHandler(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("bind panicked with route option present: %v", r)
		}
	}()

	router := velaros.NewRouter()
	// Option order should not matter.
	router.PublicBind("/api/items", func(ctx *velaros.Context) {}, velaros.WithMetadata("m"))
}

func TestWithMetadata_OptionAlonePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when binding with only a route option and no handlers")
		}
	}()

	router := velaros.NewRouter()
	router.PublicBind("/api/items", velaros.WithMetadata("m"))
}

func TestWithMetadata_PrivateBindAcceptsOption(t *testing.T) {
	router := velaros.NewRouter()
	router.Bind("/internal/thing", velaros.WithMetadata("m"), func(ctx *velaros.Context) {})

	if len(router.RouteDescriptors()) != 0 {
		t.Fatal("private routes must not produce route descriptors")
	}
}

func TestRouteDescriptor_JSONRoundTripWithMetadata(t *testing.T) {
	pattern, err := velaros.NewPattern("/a/b/c")
	if err != nil {
		t.Fatalf("failed to create pattern: %s", err)
	}

	descriptor := &velaros.RouteDescriptor{
		Pattern:  pattern,
		Metadata: map[string]any{"rateLimit": 100},
	}

	data, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("failed to marshal: %s", err)
	}

	decoded := &velaros.RouteDescriptor{}
	if err := json.Unmarshal(data, decoded); err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}

	if decoded.Pattern.String() != "/a/b/c" {
		t.Errorf("pattern mismatch: %s", decoded.Pattern.String())
	}

	// Metadata is preserved as raw JSON so consumers can decode it into
	// their own types.
	raw, ok := decoded.Metadata.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage metadata, got %T", decoded.Metadata)
	}
	var meta struct {
		RateLimit int `json:"rateLimit"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("failed to decode metadata: %s", err)
	}
	if meta.RateLimit != 100 {
		t.Errorf("expected rateLimit=100, got %d", meta.RateLimit)
	}
}

func TestRouteDescriptor_JSONRoundTripWithoutMetadata(t *testing.T) {
	pattern, err := velaros.NewPattern("/a/b/c")
	if err != nil {
		t.Fatalf("failed to create pattern: %s", err)
	}

	data, err := json.Marshal(&velaros.RouteDescriptor{Pattern: pattern})
	if err != nil {
		t.Fatalf("failed to marshal: %s", err)
	}

	decoded := &velaros.RouteDescriptor{}
	if err := json.Unmarshal(data, decoded); err != nil {
		t.Fatalf("failed to unmarshal: %s", err)
	}
	if decoded.Metadata != nil {
		t.Errorf("expected nil metadata, got %v", decoded.Metadata)
	}
}

func TestWithMetadata_NestedRouterMountPreservesMetadata(t *testing.T) {
	inner := velaros.NewRouter()
	inner.PublicBind("/items/:id", velaros.WithMetadata("inner-policy"), func(ctx *velaros.Context) {})

	outer := velaros.NewRouter()
	outer.Bind("/api/**", inner)

	descriptors := outer.RouteDescriptors()
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 route descriptor, got %d", len(descriptors))
	}
	if descriptors[0].Pattern.String() != "/api/items/:id" {
		t.Errorf("unexpected mounted pattern: %s", descriptors[0].Pattern.String())
	}
	if descriptors[0].Metadata != "inner-policy" {
		t.Errorf("expected inner metadata to survive mounting, got %v", descriptors[0].Metadata)
	}
}
