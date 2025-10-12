package velaros_test

import (
	"testing"

	"github.com/RobertWHurst/velaros"
)

func TestMessageParamsGet(t *testing.T) {
	cases := []struct {
		message       string
		params        velaros.MessageParams
		key           string
		expectedValue string
	}{
		{"should return the value for a key that exists",
			velaros.MessageParams{"Content-Type": "application/json"},
			"Content-Type", "application/json",
		},
		{"should return the value for a key that exists with different casing",
			velaros.MessageParams{"Content-Type": "application/json"},
			"content-type", "application/json",
		},
		{"should return an empty string for a key that does not exist",
			velaros.MessageParams{"Content-Type": "application/json"},
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
