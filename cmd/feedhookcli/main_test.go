package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runApp(args ...string) error {
	_, _, err := runAppWithOutput(args...)
	return err
}

func runAppWithOutput(args ...string) (stdout string, stderr string, err error) {
	var out, errOut bytes.Buffer
	app := newApp(remote.NewClient())
	app.Writer = &out
	app.ErrWriter = &errOut
	err = app.Run(append([]string{"feedhookcli"}, args...))
	return out.String(), errOut.String(), err
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
			want: "expected one argument: webhook-name, got: hook -p 2234",
		},
		{
			name: "too many arguments",
			args: []string{"check-config", "a.toml", "b.toml"},
			want: "expected one argument: path, got: a.toml b.toml",
		},
		{
			name: "unexpected argument",
			args: []string{"stats", "extra"},
			want: "expected no arguments, got: extra",
		},
		{
			name: "invalid timeout",
			args: []string{"stats", "--timeout", "0"},
			want: "invalid timeout: 0s",
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

func TestHelpOnError(t *testing.T) {
	cases := [][]string{
		{},
		{"stat"},
		{"ping"},
		{"stats", "extra"},
		{"stats", "--port", "abc"},
		{"check-config", "-x", "a.toml"},
		{"--port", "2234", "stats"},
	}
	for _, args := range cases {
		t.Run(fmt.Sprintf("should show help on stderr for %q", args), func(t *testing.T) {
			// when
			stdout, stderr, err := runAppWithOutput(args...)
			// then
			require.Error(t, err)
			assert.Empty(t, stdout)
			assert.Contains(t, stderr, "USAGE:")
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
		stdout, stderr, err := runAppWithOutput("check-config", p)
		// then
		require.NoError(t, err)
		assert.Equal(t, p+" is valid\n", stdout)
		assert.Empty(t, stderr)
	})
	t.Run("should show warnings for valid config", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		data := `
[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook"

[[webhooks]]
name = "hook-2"
url = "https://www.example.com/webhook-2"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`
		err := os.WriteFile(p, []byte(data), 0644)
		require.NoError(t, err)
		// when
		stdout, stderr, err := runAppWithOutput("check-config", p)
		// then
		require.NoError(t, err)
		assert.Equal(t, p+" is valid\n", stdout)
		assert.Equal(t, "warning: webhook \"hook-2\" defined, but not used\n", stderr)
	})
	t.Run("should show warnings and failures for invalid config", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		data := `
[[webhooks]]
name = "hook-1"
ulr = "https://www.example.com/webhook"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
`
		err := os.WriteFile(p, []byte(data), 0644)
		require.NoError(t, err)
		// when
		stdout, stderr, err := runAppWithOutput("check-config", p)
		// then
		require.EqualError(t, err, "invalid config")
		assert.Empty(t, stdout)
		want := "warning: unknown key: webhooks.ulr\n" +
			"warning: webhook \"hook-1\" defined, but not used\n" +
			"error: webhook \"hook-1\" has no url\n" +
			"error: feed \"Feed 1\" has no webhook\n"
		assert.Equal(t, want, stderr)
	})
}
