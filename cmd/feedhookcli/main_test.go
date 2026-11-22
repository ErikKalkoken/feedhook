package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validConfig = `
[[webhooks]]
name = "hook-1"
url = "https://www.example.com/webhook"

[[feeds]]
name = "Feed 1"
url = "https://www.example.com/feed.rss"
webhook = "hook-1"
`

func TestRun(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no command",
			args:    []string{},
			wantErr: "missing command",
		},
		{
			name:    "unknown command",
			args:    []string{"dummy"},
			wantErr: "unknown command: dummy",
		},
		{
			name:    "check-config without arg",
			args:    []string{"check-config"},
			wantErr: "expected one argument: path",
		},
		{
			name:    "ping with too many args",
			args:    []string{"ping", "a", "b"},
			wantErr: "expected one argument: webhook-name, got: a b",
		},
		{
			name:    "post-latest without arg",
			args:    []string{"post-latest"},
			wantErr: "expected one argument: feed-name",
		},
		{
			name:    "restart with arg",
			args:    []string{"restart", "a"},
			wantErr: "expected no arguments, got: a",
		},
		{
			name:    "stats with arg",
			args:    []string{"stats", "a"},
			wantErr: "expected no arguments, got: a",
		},
		{
			name:    "invalid port",
			args:    []string{"stats", "-port", "0"},
			wantErr: "invalid port: 0",
		},
		{
			name:    "invalid timeout",
			args:    []string{"stats", "-timeout", "-1s"},
			wantErr: "invalid timeout: -1s",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, remote.NewClient())
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestRunVersion(t *testing.T) {
	err := run([]string{"-version"}, remote.NewClient())
	assert.NoError(t, err)
}

func TestRunCheckConfig(t *testing.T) {
	t.Run("should accept valid config", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(p, []byte(validConfig), 0644))
		err := run([]string{"check-config", p}, remote.NewClient())
		assert.NoError(t, err)
	})
	t.Run("should report error for invalid config", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(p, []byte("[[webhooks]\n"), 0644))
		err := run([]string{"check-config", p}, remote.NewClient())
		assert.Error(t, err)
	})
}
