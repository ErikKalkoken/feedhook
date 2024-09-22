package dhooks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLength(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"alpha 😀 boy", 11},
		{"alpha boy", 9},
		{"", 0},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("#%d", i+1), func(t *testing.T) {
			got := length(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}
