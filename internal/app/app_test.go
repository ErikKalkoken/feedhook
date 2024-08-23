package app_test

import (
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	bolt "go.etcd.io/bbolt"

	"example/feedforward/internal/app"
)

type faketime struct {
	now time.Time
}

func (rt faketime) Now() time.Time {
	return rt.now
}

func TestApp(t *testing.T) {
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
		log.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	if err := app.SetupDB(db); err != nil {
		log.Fatalf("Failed to setup DB: %s", err)
	}
	cfg := app.MyConfig{
		App:      app.ConfigApp{Oldest: 3600 * 24},
		Webhooks: []app.ConfigWebhook{{Name: "hook1", URL: "https://www.example.com/hook"}},
		Feeds:    []app.ConfigFeed{{Name: "feed1", URL: "https://www.example.com/feed", Webhook: "hook1"}},
	}
	a := app.New(db, cfg, faketime{now: time.Date(2024, 8, 22, 12, 0, 0, 0, time.UTC)})
	a.Start()
	time.Sleep(2 * time.Second)
	a.Close()
	info := httpmock.GetCallCountInfo()
	assert.Equal(t, 1, info["POST https://www.example.com/hook"])
	assert.GreaterOrEqual(t, info["GET https://www.example.com/feed"], 1)
}
