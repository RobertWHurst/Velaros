package scramjet_test

import (
	"github.com/RobertWHurst/scramjet"
	"testing"
)

func TestMessageParamsGet(t *testing.T) {
	cases := []struct {
		message       string
		params        scramjet.MessageParams
		key           string
		expectedValue string
	}{
		{"should return the value for a key that exists",
			scramjet.MessageParams{"Content-Type": "application/json"},
			"Content-Type", "application/json",
		},
		{"should return the value for a key that exists with different casing",
			scramjet.MessageParams{"Content-Type": "application/json"},
			"content-type", "application/json",
		},
		{"should return an empty string for a key that does not exist",
			scramjet.MessageParams{"Content-Type": "application/json"},
			"Accept", "",
		},
	}

	for _, c := range cases {
		t.Run(c.message, func(t *testing.T) {
			value := c.params.Get(c.key)
			if value != c.expectedValue {
				t.Errorf("expected %s, got %s", c.expectedValue, value)
			}
		})
	}
}
