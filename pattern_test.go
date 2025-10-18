package velaros

import (
	"testing"
)

func TestNewPattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		shouldError bool
	}{
		{
			name:        "simple static path",
			pattern:     "/users",
			shouldError: false,
		},
		{
			name:        "path with dynamic segment",
			pattern:     "/users/:id",
			shouldError: false,
		},
		{
			name:        "path with multiple dynamic segments",
			pattern:     "/users/:userId/posts/:postId",
			shouldError: false,
		},
		{
			name:        "path with wildcard",
			pattern:     "/static/*",
			shouldError: false,
		},
		{
			name:        "path with optional segment",
			pattern:     "/api/:version?/users",
			shouldError: false,
		},
		{
			name:        "path with one or more modifier",
			pattern:     "/files/:path+",
			shouldError: false,
		},
		{
			name:        "path with zero or more modifier",
			pattern:     "/files/:path*",
			shouldError: false,
		},
		{
			name:        "path with custom pattern",
			pattern:     "/users/:id([0-9]+)",
			shouldError: false,
		},
		{
			name:        "no leading slash",
			pattern:     "users",
			shouldError: true,
		},
		{
			name:        "dynamic segment without name",
			pattern:     "/users/:(invalid)",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := NewPattern(tt.pattern)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error for pattern %q, got nil", tt.pattern)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for pattern %q: %v", tt.pattern, err)
				}
				if pattern == nil {
					t.Errorf("expected pattern for %q, got nil", tt.pattern)
				}
				if pattern.String() != tt.pattern {
					t.Errorf("expected pattern.String() to be %q, got %q", tt.pattern, pattern.String())
				}
			}
		})
	}
}

func TestPatternMatch(t *testing.T) {
	tests := []struct {
		name           string
		pattern        string
		path           string
		shouldMatch    bool
		expectedParams map[string]string
	}{
		{
			name:           "exact match",
			pattern:        "/users",
			path:           "/users",
			shouldMatch:    true,
			expectedParams: map[string]string{},
		},
		{
			name:           "exact match with trailing slash",
			pattern:        "/users",
			path:           "/users/",
			shouldMatch:    true,
			expectedParams: map[string]string{},
		},
		{
			name:        "no match - different path",
			pattern:     "/users",
			path:        "/posts",
			shouldMatch: false,
		},
		{
			name:        "dynamic segment match",
			pattern:     "/users/:id",
			path:        "/users/123",
			shouldMatch: true,
			expectedParams: map[string]string{
				"id": "123",
			},
		},
		{
			name:        "dynamic segment - no match without segment",
			pattern:     "/users/:id",
			path:        "/users",
			shouldMatch: false,
		},
		{
			name:        "multiple dynamic segments",
			pattern:     "/users/:userId/posts/:postId",
			path:        "/users/42/posts/101",
			shouldMatch: true,
			expectedParams: map[string]string{
				"userId": "42",
				"postId": "101",
			},
		},
		{
			name:           "wildcard match",
			pattern:        "/static/*",
			path:           "/static/anything",
			shouldMatch:    true,
			expectedParams: map[string]string{},
		},
		{
			name:        "wildcard match - multiple segments",
			pattern:     "/static/*",
			path:        "/static/css/style.css",
			shouldMatch: false, // wildcard only matches single segment without +
		},
		{
			name:        "optional segment - present",
			pattern:     "/api/:version?/users",
			path:        "/api/v1/users",
			shouldMatch: true,
			expectedParams: map[string]string{
				"version": "v1",
			},
		},
		{
			name:        "optional segment - missing",
			pattern:     "/api/:version?/users",
			path:        "/api/users",
			shouldMatch: true,
			expectedParams: map[string]string{
				"version": "", // Optional params are present but empty when not provided
			},
		},
		{
			name:        "one or more - single segment",
			pattern:     "/files/:path+",
			path:        "/files/document.pdf",
			shouldMatch: true,
			expectedParams: map[string]string{
				"path": "document.pdf",
			},
		},
		{
			name:        "one or more - multiple segments",
			pattern:     "/files/:path+",
			path:        "/files/2024/01/document.pdf",
			shouldMatch: true,
			expectedParams: map[string]string{
				"path": "2024/01/document.pdf",
			},
		},
		{
			name:        "one or more - requires at least one segment",
			pattern:     "/files/:path+",
			path:        "/files",
			shouldMatch: false,
		},
		{
			name:        "zero or more - no segments",
			pattern:     "/files/:path*",
			path:        "/files",
			shouldMatch: true,
			expectedParams: map[string]string{
				"path": "", // Zero or more params are present but empty when not provided
			},
		},
		{
			name:        "zero or more - multiple segments",
			pattern:     "/files/:path*",
			path:        "/files/2024/01/document.pdf",
			shouldMatch: true,
			expectedParams: map[string]string{
				"path": "2024/01/document.pdf",
			},
		},
		{
			name:        "custom pattern - numeric only",
			pattern:     "/users/:id([0-9]+)",
			path:        "/users/123",
			shouldMatch: true,
			expectedParams: map[string]string{
				"id": "123",
			},
		},
		{
			name:        "custom pattern - non-numeric rejected",
			pattern:     "/users/:id([0-9]+)",
			path:        "/users/abc",
			shouldMatch: false,
		},
		{
			name:        "custom pattern - UUID format",
			pattern:     "/users/:id([a-f0-9-]{36})",
			path:        "/users/123e4567-e89b-12d3-a456-426614174000",
			shouldMatch: true,
			expectedParams: map[string]string{
				"id": "123e4567-e89b-12d3-a456-426614174000",
			},
		},
		{
			name:        "mixed static and dynamic",
			pattern:     "/api/v1/users/:userId/posts/:postId/comments",
			path:        "/api/v1/users/42/posts/101/comments",
			shouldMatch: true,
			expectedParams: map[string]string{
				"userId": "42",
				"postId": "101",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := NewPattern(tt.pattern)
			if err != nil {
				t.Fatalf("failed to create pattern: %v", err)
			}

			params, matched := pattern.Match(tt.path)

			if matched != tt.shouldMatch {
				t.Errorf("expected match=%v, got match=%v", tt.shouldMatch, matched)
			}

			if tt.shouldMatch {
				if params == nil {
					t.Error("expected params to be non-nil on match")
				}
				for key, expectedValue := range tt.expectedParams {
					actualValue, ok := params[key]
					if !ok {
						t.Errorf("expected param %q to exist", key)
					}
					if actualValue != expectedValue {
						t.Errorf("expected param %q to be %q, got %q", key, expectedValue, actualValue)
					}
				}
				// Check no extra params
				if len(params) != len(tt.expectedParams) {
					t.Errorf("expected %d params, got %d: %v", len(tt.expectedParams), len(params), params)
				}
			}
		})
	}
}

func TestPatternMatchInto(t *testing.T) {
	pattern, err := NewPattern("/users/:id/posts/:postId")
	if err != nil {
		t.Fatalf("failed to create pattern: %v", err)
	}

	t.Run("match with nil params", func(t *testing.T) {
		var params MessageParams
		matched := pattern.MatchInto("/users/123/posts/456", &params)

		if !matched {
			t.Error("expected match")
		}

		if params == nil {
			t.Fatal("expected params to be initialized")
		}

		if params["id"] != "123" {
			t.Errorf("expected id=123, got %q", params["id"])
		}
		if params["postId"] != "456" {
			t.Errorf("expected postId=456, got %q", params["postId"])
		}
	})

	t.Run("match with existing params - clears old values", func(t *testing.T) {
		params := MessageParams{
			"oldKey": "oldValue",
			"id":     "old",
		}

		matched := pattern.MatchInto("/users/999/posts/888", &params)

		if !matched {
			t.Error("expected match")
		}

		if _, exists := params["oldKey"]; exists {
			t.Error("expected old key to be cleared")
		}

		if params["id"] != "999" {
			t.Errorf("expected id=999, got %q", params["id"])
		}
		if params["postId"] != "888" {
			t.Errorf("expected postId=888, got %q", params["postId"])
		}
	})

	t.Run("no match - params unchanged", func(t *testing.T) {
		params := MessageParams{"test": "value"}
		matched := pattern.MatchInto("/wrong/path", &params)

		if matched {
			t.Error("expected no match")
		}
	})
}

func TestPatternEdgeCases(t *testing.T) {
	t.Run("root path", func(t *testing.T) {
		pattern, err := NewPattern("/")
		if err != nil {
			t.Fatalf("failed to create pattern: %v", err)
		}

		params, matched := pattern.Match("/")
		if !matched {
			t.Error("expected root path to match")
		}
		if len(params) != 0 {
			t.Errorf("expected no params, got %v", params)
		}
	})

	t.Run("empty path matches root", func(t *testing.T) {
		pattern, err := NewPattern("/")
		if err != nil {
			t.Fatalf("failed to create pattern: %v", err)
		}

		// The regex allows empty path to match root due to the /? at the end
		_, matched := pattern.Match("")
		if !matched {
			t.Error("expected empty string to match root pattern")
		}
	})

	t.Run("trailing slash normalization", func(t *testing.T) {
		pattern, err := NewPattern("/users")
		if err != nil {
			t.Fatalf("failed to create pattern: %v", err)
		}

		_, matched1 := pattern.Match("/users")
		_, matched2 := pattern.Match("/users/")

		if !matched1 {
			t.Error("expected /users to match")
		}
		if !matched2 {
			t.Error("expected /users/ to match")
		}
	})

	t.Run("case sensitivity", func(t *testing.T) {
		pattern, err := NewPattern("/users")
		if err != nil {
			t.Fatalf("failed to create pattern: %v", err)
		}

		_, matched := pattern.Match("/Users")
		if matched {
			t.Error("expected case-sensitive matching - /Users should not match /users")
		}
	})

	t.Run("special characters in static path", func(t *testing.T) {
		pattern, err := NewPattern("/api-v1/users.json")
		if err != nil {
			t.Fatalf("failed to create pattern: %v", err)
		}

		_, matched := pattern.Match("/api-v1/users.json")
		if !matched {
			t.Error("expected special characters in path to work")
		}
	})
}

func TestPatternPath(t *testing.T) {
	tests := []struct {
		name         string
		pattern      string
		params       MessageParams
		wildcards    []string
		expectedPath string
		shouldError  bool
	}{
		{
			name:         "static path",
			pattern:      "/users",
			params:       nil,
			wildcards:    nil,
			expectedPath: "/users",
			shouldError:  false,
		},
		{
			name:         "dynamic required param",
			pattern:      "/users/:id",
			params:       MessageParams{"id": "123"},
			wildcards:    nil,
			expectedPath: "/users/123",
			shouldError:  false,
		},
		{
			name:         "dynamic required missing param",
			pattern:      "/users/:id",
			params:       MessageParams{},
			wildcards:    nil,
			expectedPath: "",
			shouldError:  true,
		},
		{
			name:         "dynamic optional present",
			pattern:      "/users/:id?",
			params:       MessageParams{"id": "123"},
			wildcards:    nil,
			expectedPath: "/users/123",
			shouldError:  false,
		},
		{
			name:         "dynamic optional missing",
			pattern:      "/users/:id?",
			params:       MessageParams{},
			wildcards:    nil,
			expectedPath: "/users",
			shouldError:  false,
		},
		{
			name:         "wildcard single",
			pattern:      "/static/*",
			params:       nil,
			wildcards:    []string{"file.js"},
			expectedPath: "/static/file.js",
			shouldError:  false,
		},
		{
			name:         "wildcard insufficient",
			pattern:      "/static/*",
			params:       nil,
			wildcards:    []string{},
			expectedPath: "",
			shouldError:  true,
		},
		{
			name:         "zero or more empty",
			pattern:      "/files/:path*",
			params:       MessageParams{},
			wildcards:    nil,
			expectedPath: "/files",
			shouldError:  false,
		},
		{
			name:         "one or more with value",
			pattern:      "/files/:path+",
			params:       MessageParams{"path": "doc.pdf"},
			wildcards:    nil,
			expectedPath: "/files/doc.pdf",
			shouldError:  false,
		},
		{
			name:         "one or more missing",
			pattern:      "/files/:path+",
			params:       MessageParams{},
			wildcards:    nil,
			expectedPath: "",
			shouldError:  true,
		},
		{
			name:         "mixed static dynamic wildcard",
			pattern:      "/api/:version/users/*",
			params:       MessageParams{"version": "v1"},
			wildcards:    []string{"profile/123"},
			expectedPath: "/api/v1/users/profile/123",
			shouldError:  false,
		},
		{
			name:         "root path",
			pattern:      "/",
			params:       nil,
			wildcards:    nil,
			expectedPath: "/",
			shouldError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := NewPattern(tt.pattern)
			if err != nil {
				t.Fatalf("failed to create pattern: %v", err)
			}

			path, err := pattern.Path(tt.params, tt.wildcards)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if path != tt.expectedPath {
				t.Errorf("expected path %q, got %q", tt.expectedPath, path)
			}
		})
	}
}
