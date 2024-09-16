package service_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app"
	"github.com/ErikKalkoken/feedhook/internal/app/service"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
)

type faketime struct {
	now time.Time
}

func (rt faketime) Now() time.Time {
	return rt.now
}

func TestService(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
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
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	cfg := app.MyConfig{
		App:      app.ConfigApp{Oldest: 3600 * 24, Ticker: 1},
		Webhooks: []app.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []app.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhooks: []string{"hook1"}}},
	}
	st := storage.New(db, cfg)
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
	s := service.NewService(st, cfg, faketime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	s.Start()
	time.Sleep(2 * time.Second)
	s.Close()
	info := httpmock.GetCallCountInfo()
	assert.Equal(t, 1, info["POST https://www.example.com/hook"])
	assert.LessOrEqual(t, 1, info["GET https://www.example.com/feed"])
	fs, err := st.GetFeedStats("feed1")
	if assert.NoError(t, err) {
		assert.Equal(t, 1, fs.ReceivedCount)
	}
}
