package dispatcher_test

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
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

func TestService(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cfg := config.Config{
		App:      config.ConfigApp{Oldest: 3600 * 24, Ticker: 1},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []config.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}},
	}
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
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
		if assert.NoError(t, err) {
			assert.Equal(t, 1, fs.ReceivedCount)
		}
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

func TestFetchFeedUsesConfiguredTimeout(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cfg := config.Config{
		App:      config.ConfigApp{Timeout: 120},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []config.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}},
	}
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
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
	err = d.PostLatestFeedItem("feed1")
	if assert.NoError(t, err) && assert.True(t, hasDeadline) {
		assert.WithinDuration(t, start.Add(120*time.Second), deadline, 5*time.Second)
	}
}

func TestSendPing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cfg := config.Config{
		App:      config.ConfigApp{Timeout: 120},
		Webhooks: []config.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
	}
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
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
		if assert.NoError(t, err) {
			info := httpmock.GetCallCountInfo()
			assert.Equal(t, 1, info["POST https://www.example.com/hook"])
			if assert.True(t, hasDeadline) {
				assert.WithinDuration(t, start.Add(120*time.Second), deadline, 5*time.Second)
			}
		}
	})
	t.Run("should return not found for unknown webhook", func(t *testing.T) {
		httpmock.Reset()
		err := d.SendPing("unknown")
		assert.True(t, errors.Is(err, dispatcher.ErrNotFound))
	})
}
