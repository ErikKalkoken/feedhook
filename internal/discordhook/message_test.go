package discordhook_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/discordhook"
	"github.com/stretchr/testify/assert"
)

func TestMessageValidate(t *testing.T) {
	cases := []struct {
		m  discordhook.Message
		ok bool
	}{
		{discordhook.Message{Content: "content"}, true},
		{discordhook.Message{}, false},
		{discordhook.Message{Embeds: []discordhook.Embed{{Description: "description"}}}, true},
		{discordhook.Message{Embeds: []discordhook.Embed{{Timestamp: "invalid"}}}, false},
		{discordhook.Message{Embeds: []discordhook.Embed{{Timestamp: "2006-01-02T15:04:05Z"}}}, true},
		{discordhook.Message{Content: makeStr(2001)}, false},
		{discordhook.Message{Embeds: []discordhook.Embed{{Description: makeStr(4097)}}}, false},
		{
			discordhook.Message{Embeds: []discordhook.Embed{
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
