package dispatcher_test

import (
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

type faketime struct {
	now time.Time
}

func (rt faketime) Now() time.Time {
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
		d := dispatcher.New(st, cfg, faketime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
		if err := d.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		d.Close()
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
		d := dispatcher.New(st, cfg, faketime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
		if err := d.Start(); err != nil {
			t.Fatal(err)
		}
		err := d.Start()
		assert.Error(t, err)
		d.Close()
		err = d.Start()
		assert.NoError(t, err)
		assert.True(t, d.Close())
		assert.False(t, d.Close())
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
		d := dispatcher.New(st, cfg, faketime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
		if err := d.Start(); err != nil {
			t.Fatal(err)
		}
		err := d.Restart()
		assert.NoError(t, err)
		assert.True(t, d.Close())
	})
}
