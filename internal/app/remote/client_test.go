package remote_test

import (
	"net"
	"net/http/httptest"
	"net/rpc"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/dispatcher"
	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

const validConfig = `
[[webhooks]]
name = "hook1"
url = "https://www.example.com/hook1"

[[feeds]]
name = "feed1"
url = "https://www.example.com/feed1"
webhook = "hook1"
`

func TestClientCallsRemoteService(t *testing.T) {
	// The config has no webhooks, so the dispatcher can not start and no HTTP requests are made.
	cfg := config.Config{
		Feeds: []config.ConfigFeed{
			{
				Name:    "feed1",
				URL:     "https://www.example.com/feed1",
				Webhook: "hook1",
			},
		},
	}
	t.Run("check config reports valid config", func(t *testing.T) {
		// given
		configPath := writeFile(t, validConfig)
		c := newClientWithServer(t, cfg, configPath)
		// when
		err := c.CheckConfig()
		// then
		assert.NoError(t, err)
	})
	t.Run("check config reports invalid config", func(t *testing.T) {
		// given
		configPath := writeFile(t, "")
		c := newClientWithServer(t, cfg, configPath)
		// when
		err := c.CheckConfig()
		// then
		assert.ErrorContains(t, err, "no feeds defined")
	})
	t.Run("statistics", func(t *testing.T) {
		// given
		c := newClientWithServer(t, cfg, "")
		// when
		got, err := c.Statistics()
		// then
		require.NoError(t, err)
		assert.Contains(t, got, "    feed1")
	})
	t.Run("post latest feed item passes feed name", func(t *testing.T) {
		// given
		c := newClientWithServer(t, cfg, "")
		// when
		err := c.PostLatestFeedItem("unknown")
		// then
		assert.ErrorContains(t, err, `feed "unknown"`)
	})
	t.Run("send ping passes webhook name", func(t *testing.T) {
		// given
		c := newClientWithServer(t, cfg, "")
		// when
		err := c.SendPing("unknown")
		// then
		assert.ErrorContains(t, err, `webhook "unknown"`)
	})
	t.Run("restart", func(t *testing.T) {
		// given
		c := newClientWithServer(t, cfg, "")
		// when
		err := c.Restart()
		// then
		assert.ErrorContains(t, err, "no webhook defined")
	})
}

func TestClientSetPort(t *testing.T) {
	t.Run("accepts valid ports", func(t *testing.T) {
		for _, port := range []int{1, 65535} {
			// given
			c := remote.NewClient()
			// when
			err := c.SetPort(port)
			// then
			assert.NoError(t, err, port)
		}
	})
	t.Run("rejects invalid ports and keeps current port", func(t *testing.T) {
		// given
		c := newClientWithServer(t, config.Config{}, "")
		ports := []int{-1, 0, 65536}
		// when
		errs := make([]error, len(ports))
		for i, port := range ports {
			errs[i] = c.SetPort(port)
		}
		// then
		for i, err := range errs {
			assert.ErrorContains(t, err, "invalid port", ports[i])
		}
		_, err := c.Statistics()
		assert.NoError(t, err, "client should still use the old port")
	})
}

func TestClientReportsErrorWhenServiceNotRunning(t *testing.T) {
	// given
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	c := remote.NewClient()
	require.NoError(t, c.SetPort(port))
	// when
	_, err = c.Statistics()
	// then
	assert.ErrorContains(t, err, "dialing")
}

// newClientWithServer starts a remote service for cfg and returns a client connected to it.
func newClientWithServer(t *testing.T, cfg config.Config, configPath string) *remote.Client {
	t.Helper()
	db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0600, nil)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	st := storage.New(db)
	require.NoError(t, st.Init(cfg.FeedNames()))
	d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	srv := rpc.NewServer()
	require.NoError(t, srv.Register(remote.NewRemoteService(d, st, cfg, configPath)))
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	c := remote.NewClient()
	require.NoError(t, c.SetPort(ts.Listener.Addr().(*net.TCPAddr).Port))
	return c
}

// writeFile writes content to a temporary file and returns its path.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0600))
	return p
}
