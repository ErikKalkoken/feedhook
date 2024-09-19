package discordhook

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage(t *testing.T) {
	cases := []struct {
		m  Message
		ok bool
	}{
		{Message{Content: "content"}, true},
		{Message{}, false},
		{Message{Embeds: []Embed{{Description: "description"}}}, true},
		{Message{Embeds: []Embed{{Timestamp: "invalid"}}}, false},
		{Message{Embeds: []Embed{{Timestamp: "2006-01-02T15:04:05Z"}}}, true},
		{Message{Content: makeStr(2001)}, false},
		{Message{Embeds: []Embed{{Description: makeStr(4097)}}}, false},
		{
			Message{Embeds: []Embed{
				{Description: makeStr(4096)},
				{Description: makeStr(4096)},
			}},
			false,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(i+1), func(t *testing.T) {
			err := tc.m.validate()
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
