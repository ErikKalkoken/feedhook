package dhook_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/dhook"
	"github.com/stretchr/testify/assert"
)

func TestMessageValidate(t *testing.T) {
	cases := []struct {
		m  dhook.Message
		ok bool
	}{
		{dhook.Message{Content: "content"}, true},
		{dhook.Message{}, false},
		{dhook.Message{Embeds: []dhook.Embed{{Description: "description"}}}, true},
		{dhook.Message{Embeds: []dhook.Embed{{Timestamp: "invalid"}}}, false},
		{dhook.Message{Embeds: []dhook.Embed{{Timestamp: "2006-01-02T15:04:05Z"}}}, true},
		{dhook.Message{Content: makeStr(2001)}, false},
		{dhook.Message{Embeds: []dhook.Embed{{Description: makeStr(4097)}}}, false},
		{
			dhook.Message{Embeds: []dhook.Embed{
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
