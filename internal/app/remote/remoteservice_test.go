package remote_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/dispatcher"
	"github.com/ErikKalkoken/feedhook/internal/app/remote"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

type fakeTime struct {
	now time.Time
}

func (rt fakeTime) Now() time.Time {
	return rt.now
}

func TestStatisticsSortsByNameWithoutChangingConfig(t *testing.T) {
	// given
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cfg := config.Config{
		Webhooks: []config.ConfigWebhook{
			{
				Name: "hook2",
				URL:  "https://www.example.com/hook2",
			},
			{
				Name: "hook1",
				URL:  "https://www.example.com/hook1",
			},
		},
		Feeds: []config.ConfigFeed{
			{
				Name:    "feed2",
				URL:     "https://www.example.com/feed2",
				Webhook: "hook2",
			},
			{
				Name:    "feed1",
				URL:     "https://www.example.com/feed1",
				Webhook: "hook1",
			},
		},
	}
	st := storage.New(db)
	err = st.Init(cfg.FeedNames())
	require.NoError(t, err)
	for _, cf := range cfg.Feeds {
		if err := st.UpdateOrCreateFeedStats(cf.Name, func(fs *app.FeedStats) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	for _, cw := range cfg.Webhooks {
		if err := st.UpdateOrCreateWebhookStats(cw.Name, func(ws *app.WebhookStats) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	s := remote.NewRemoteService(d, st, cfg, "")
	// when
	var reply string
	err = s.Statistics(&remote.EmptyArgs{}, &reply)
	// then
	require.NoError(t, err)
	assert.Equal(t, "feed2", cfg.Feeds[0].Name)
	assert.Equal(t, "feed1", cfg.Feeds[1].Name)
	assert.Equal(t, "hook2", cfg.Webhooks[0].Name)
	assert.Equal(t, "hook1", cfg.Webhooks[1].Name)
	feedsOut, hooksOut, ok := strings.Cut(reply, "Webhooks:\n")
	require.True(t, ok)
	for _, s := range []string{"    feed1", "    feed2"} {
		assert.Contains(t, feedsOut, s)
	}
	for _, s := range []string{"    hook1", "    hook2"} {
		assert.Contains(t, hooksOut, s)
	}
	assert.Less(t, strings.Index(feedsOut, "    feed1"), strings.Index(feedsOut, "    feed2"))
	assert.Less(t, strings.Index(hooksOut, "    hook1"), strings.Index(hooksOut, "    hook2"))
}

func TestStatisticsListsFeedsAndWebhooksWithoutStats(t *testing.T) {
	// given
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	require.NoError(t, err)
	defer db.Close()
	cfg := config.Config{
		Webhooks: []config.ConfigWebhook{
			{
				Name: "hook1",
				URL:  "https://www.example.com/hook1",
			},
			{
				Name: "hook2",
				URL:  "https://www.example.com/hook2",
			},
		},
		Feeds: []config.ConfigFeed{
			{
				Name:    "feed1",
				URL:     "https://www.example.com/feed1",
				Webhook: "hook1",
			},
			{
				Name:    "feed2",
				URL:     "https://www.example.com/feed2",
				Webhook: "hook2",
			},
		},
	}
	st := storage.New(db)
	err = st.Init(cfg.FeedNames())
	require.NoError(t, err)
	// only feed1 and hook1 have stats
	err = st.UpdateOrCreateFeedStats("feed1", func(fs *app.FeedStats) error { return nil })
	require.NoError(t, err)
	err = st.UpdateOrCreateWebhookStats("hook1", func(ws *app.WebhookStats) error { return nil })
	require.NoError(t, err)
	d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	s := remote.NewRemoteService(d, st, cfg, "")
	// when
	var reply string
	assert.NotPanics(t, func() {
		err = s.Statistics(&remote.EmptyArgs{}, &reply)
	})
	// then
	require.NoError(t, err)
	feedsOut, hooksOut, ok := strings.Cut(reply, "Webhooks:\n")
	require.True(t, ok)
	assert.Contains(t, feedsOut, "    feed1")
	assert.Contains(t, feedsOut, "    feed2")
	assert.Contains(t, hooksOut, "    hook1")
	assert.Contains(t, hooksOut, "    hook2")
}

func TestCheckConfig(t *testing.T) {
	t.Run("returns no error for valid config", func(t *testing.T) {
		// given
		s := remote.NewRemoteService(nil, nil, config.Config{}, writeFile(t, validConfig))
		// when
		err := s.CheckConfig(&remote.EmptyArgs{}, new(bool))
		// then
		assert.NoError(t, err)
	})
	t.Run("returns error when config file is missing", func(t *testing.T) {
		// given
		p := filepath.Join(t.TempDir(), "config.toml")
		s := remote.NewRemoteService(nil, nil, config.Config{}, p)
		// when
		err := s.CheckConfig(&remote.EmptyArgs{}, new(bool))
		// then
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
	t.Run("returns error when config file is not valid TOML", func(t *testing.T) {
		// given
		s := remote.NewRemoteService(nil, nil, config.Config{}, writeFile(t, "[[feeds]\nname ="))
		// when
		err := s.CheckConfig(&remote.EmptyArgs{}, new(bool))
		// then
		assert.Error(t, err)
	})
	t.Run("returns error when config is invalid", func(t *testing.T) {
		// given
		const cfg = `
[[webhooks]]
name = "hook1"
url = "https://www.example.com/hook1"

[[feeds]]
name = "feed1"
url = "https://www.example.com/feed1"
webhook = "unknown"
`
		s := remote.NewRemoteService(nil, nil, config.Config{}, writeFile(t, cfg))
		// when
		err := s.CheckConfig(&remote.EmptyArgs{}, new(bool))
		// then
		assert.ErrorContains(t, err, `invalid webhook "unknown"`)
	})
}
