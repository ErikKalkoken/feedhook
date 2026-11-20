package dispatcher_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/dispatcher"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

type fakeTime struct {
	now time.Time
}

func (rt fakeTime) Now() time.Time {
	return rt.now
}

// newTestStorage returns an initialized storage backed by a temporary DB.
func newTestStorage(t *testing.T, cfg config.Config) *storage.Storage {
	t.Helper()
	db, err := bolt.Open(filepath.Join(t.TempDir(), "test.db"), 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	t.Cleanup(func() { db.Close() })
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
	return st
}

func TestService(t *testing.T) {
	cfg := config.Config{
		App:      config.ConfigApp{Oldest: 3600 * 24, Ticker: 1},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []config.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}},
	}
	st := newTestStorage(t, cfg)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	t.Run("can receive item from feed and send it to configured hook", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder(
			"GET",
			"https://www.example.com/feed",
			httpmock.NewXmlResponderOrPanic(200, httpmock.File("testdata/atomfeed.xml")),
		)
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com/hook",
			httpmock.NewStringResponder(204, ""),
		)
		d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
		if err := d.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		d.Stop()
		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info["POST https://www.example.com/hook"])
		assert.LessOrEqual(t, 1, info["GET https://www.example.com/feed"])
		fs, err := st.GetFeedStats("feed1")
		require.NoError(t, err)
		assert.Equal(t, 1, fs.ReceivedCount)
	})
	t.Run("should return error when trying to start an already running dispatcher", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder(
			"GET",
			"https://www.example.com/feed",
			httpmock.NewXmlResponderOrPanic(200, httpmock.File("testdata/atomfeed.xml")),
		)
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com/hook",
			httpmock.NewStringResponder(204, ""),
		)
		d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
		if err := d.Start(); err != nil {
			t.Fatal(err)
		}
		err := d.Start()
		assert.Error(t, err)
		d.Stop()
		err = d.Start()
		assert.NoError(t, err)
		assert.True(t, d.Stop())
		assert.False(t, d.Stop())
	})
	t.Run("should restart dispatcher", func(t *testing.T) {
		httpmock.Reset()
		httpmock.RegisterResponder(
			"GET",
			"https://www.example.com/feed",
			httpmock.NewXmlResponderOrPanic(200, httpmock.File("testdata/atomfeed.xml")),
		)
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com/hook",
			httpmock.NewStringResponder(204, ""),
		)
		d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
		if err := d.Start(); err != nil {
			t.Fatal(err)
		}
		err := d.Restart()
		assert.NoError(t, err)
		assert.True(t, d.Stop())
	})
}

func TestStartFailure(t *testing.T) {
	cfg := config.Config{
		App: config.ConfigApp{Ticker: 1},
		// the empty name is not a valid bucket name, so creating the second queue fails
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook1"}, {Name: "", URL: "https://www.example.com/hook2"}},
	}
	st := newTestStorage(t, cfg)
	d := dispatcher.New(st, cfg, fakeTime{now: time.Now()})
	err := d.Start()
	assert.Error(t, err)
	done := make(chan bool)
	go func() {
		done <- d.Stop()
	}()
	select {
	case ok := <-done:
		assert.False(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not return in time")
	}
}

func TestFetchFeedUsesConfiguredTimeout(t *testing.T) {
	cfg := config.Config{
		App:      config.ConfigApp{Timeout: 120},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []config.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}},
	}
	st := newTestStorage(t, cfg)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	var deadline time.Time
	var hasDeadline bool
	feedResponder := httpmock.NewXmlResponderOrPanic(200, httpmock.File("testdata/atomfeed.xml"))
	httpmock.RegisterResponder(
		"GET",
		"https://www.example.com/feed",
		func(req *http.Request) (*http.Response, error) {
			deadline, hasDeadline = req.Context().Deadline()
			return feedResponder(req)
		},
	)
	httpmock.RegisterResponder(
		"POST",
		"https://www.example.com/hook",
		httpmock.NewStringResponder(204, ""),
	)
	d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	start := time.Now()
	err := d.PostLatestFeedItem("feed1")
	require.NoError(t, err)
	require.True(t, hasDeadline)
	assert.WithinDuration(t, start.Add(120*time.Second), deadline, 5*time.Second)
}

func TestPostLatestFeedItemRespectsBrandingDisabled(t *testing.T) {
	cfg := config.Config{
		App:      config.ConfigApp{BrandingDisabled: true},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []config.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}},
	}
	st := newTestStorage(t, cfg)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder(
		"GET",
		"https://www.example.com/feed",
		httpmock.NewXmlResponderOrPanic(200, httpmock.File("testdata/atomfeed.xml")),
	)
	var body map[string]any
	httpmock.RegisterResponder(
		"POST",
		"https://www.example.com/hook",
		func(req *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return nil, err
			}
			return httpmock.NewStringResponse(204, ""), nil
		},
	)
	d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	err := d.PostLatestFeedItem("feed1")
	require.NoError(t, err)
	assert.NotContains(t, body, "username")
	assert.NotContains(t, body, "avatar_url")
}

func TestStopCancelsFeedFetch(t *testing.T) {
	cfg := config.Config{
		App:      config.ConfigApp{Ticker: 1, Timeout: 120},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []config.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}},
	}
	st := newTestStorage(t, cfg)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	started := make(chan struct{}, 1)
	httpmock.RegisterResponder(
		"GET",
		"https://www.example.com/feed",
		func(req *http.Request) (*http.Response, error) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		},
	)
	d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for feed fetch")
	}
	done := make(chan bool)
	go func() {
		done <- d.Stop()
	}()
	select {
	case ok := <-done:
		assert.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("stop did not return in time")
	}
}

func TestSendPing(t *testing.T) {
	cfg := config.Config{
		App:      config.ConfigApp{Timeout: 120},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
	}
	st := newTestStorage(t, cfg)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	d := dispatcher.New(st, cfg, fakeTime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	t.Run("should send ping to known webhook", func(t *testing.T) {
		httpmock.Reset()
		var deadline time.Time
		var hasDeadline bool
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com/hook",
			func(req *http.Request) (*http.Response, error) {
				deadline, hasDeadline = req.Context().Deadline()
				return httpmock.NewStringResponse(204, ""), nil
			},
		)
		start := time.Now()
		err := d.SendPing("hook1")
		require.NoError(t, err)
		info := httpmock.GetCallCountInfo()
		assert.Equal(t, 1, info["POST https://www.example.com/hook"])
		require.True(t, hasDeadline)
		assert.WithinDuration(t, start.Add(120*time.Second), deadline, 5*time.Second)
	})
	t.Run("should return not found for unknown webhook", func(t *testing.T) {
		httpmock.Reset()
		err := d.SendPing("unknown")
		assert.True(t, errors.Is(err, dispatcher.ErrNotFound))
	})
}
