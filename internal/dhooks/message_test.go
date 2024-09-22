package dhooks_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/dhooks"
	"github.com/stretchr/testify/assert"
)

func TestMessageValidate(t *testing.T) {
	cases := []struct {
		m  dhooks.Message
		ok bool
	}{
		{dhooks.Message{Content: "content"}, true},
		{dhooks.Message{}, false},
		{dhooks.Message{Embeds: []dhooks.Embed{{Description: "description"}}}, true},
		{dhooks.Message{Embeds: []dhooks.Embed{{Timestamp: "invalid"}}}, false},
		{dhooks.Message{Embeds: []dhooks.Embed{{Timestamp: "2006-01-02T15:04:05Z"}}}, true},
		{dhooks.Message{Content: makeStr(2001)}, false},
		{dhooks.Message{Embeds: []dhooks.Embed{{Description: makeStr(4097)}}}, false},
		{
			dhooks.Message{Embeds: []dhooks.Embed{
				{Description: makeStr(4096)},
				{Description: makeStr(4096)},
			}},
			false,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("validate message #%d", i+1), func(t *testing.T) {
			err := tc.m.Validate()
			if tc.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func makeStr(n int) string {
	return strings.Repeat("x", n)
}
