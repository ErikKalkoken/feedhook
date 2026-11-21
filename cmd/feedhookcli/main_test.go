package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runApp(args ...string) error {
	app := newApp(remote.NewClient())
	app.Writer = io.Discard
	app.ErrWriter = io.Discard
	return app.Run(append([]string{"feedhookcli"}, args...))
}

func TestAppErrors(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing command",
			args: []string{},
			want: "missing command",
		},
		{
			name: "unknown command",
			args: []string{"stat"},
			want: "unknown command: stat",
		},
		{
			name: "missing argument",
			args: []string{"ping"},
			want: "expected one argument: webhook-name",
		},
		{
			name: "option after argument",
			args: []string{"ping", "hook", "-p", "2234"},
			want: "expected one argument: webhook-name (options must come before arguments)",
		},
		{
			name: "unexpected argument",
			args: []string{"stats", "extra"},
			want: "expected no arguments (options must come before arguments)",
		},
		{
			name: "config path is a directory",
			args: []string{"check-config", dir},
			want: dir + " is a directory, expected path to config file",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runApp(tc.args...)
			assert.EqualError(t, err, tc.want)
		})
	}
}

func TestCheckConfig(t *testing.T) {
	t.Run("should accept valid config", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		data := `
[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`
		err := os.WriteFile(p, []byte(data), 0644)
		require.NoError(t, err)
		// when
		err = runApp("check-config", p)
		// then
		assert.NoError(t, err)
	})
}
