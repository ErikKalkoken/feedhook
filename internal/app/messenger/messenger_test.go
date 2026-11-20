package messenger_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/ErikKalkoken/feedhook/internal/app/config"
	"github.com/ErikKalkoken/feedhook/internal/app/messenger"
	"github.com/ErikKalkoken/feedhook/internal/app/storage"
	"github.com/ErikKalkoken/feedhook/internal/pqueue"
	"github.com/ErikKalkoken/go-dhook"
	"github.com/jarcoal/httpmock"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessengerBacksOffOnQueueError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	q, err := pqueue.New(db, "test")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	st := storage.New(db, config.Config{})
	db.Close() // every queue access fails from now on
	h := &countingHandler{prefix: "Failed to process queue item"}
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(h))
	mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, config.Config{})
	if err := mg.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	done := make(chan bool)
	go func() {
		done <- mg.Shutdown()
	}()
	select {
	case ok := <-done:
		assert.True(t, ok)
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not return in time")
	}
	assert.Equal(t, int64(1), h.count.Load())
}

// countingHandler is a slog handler that counts the records with a message starting with prefix.
type countingHandler struct {
	prefix string
	count  atomic.Int64
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *countingHandler) Handle(_ context.Context, r slog.Record) error {
	if strings.HasPrefix(r.Message, h.prefix) {
		h.count.Add(1)
	}
	return nil
}

func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *countingHandler) WithGroup(string) slog.Handler { return h }

func TestMessenger(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open DB: %s", err)
	}
	defer db.Close()
	q, err := pqueue.New(db, "test")
	if err != nil {
		t.Fatalf("Failed to create queue: %s", err)
	}
	st := storage.New(db, config.Config{})
	if err := st.Init(); err != nil {
		t.Fatalf("Failed to init: %s", err)
	}
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	c := dhook.NewClient()
	t.Run("can return name", func(t *testing.T) {
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		assert.Equal(t, "dummy", mg.Name())

	})
	t.Run("can submit messages", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		httpmock.RegisterResponder(
			"POST",
			"https://www.example.com",
			httpmock.NewStringResponder(204, ""),
		)
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		item := &gofeed.Item{Content: "content", PublishedParsed: &now}
		err = mg.AddMessage("dummy", feed, item, false)
		time.Sleep(2 * time.Second)
		require.NoError(t, err)
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
		ws, err := st.GetWebhookStats("dummy")
		require.NoError(t, err)
		assert.Equal(t, 1, ws.SentCount)
		mg.Shutdown()
	})
	t.Run("can submit messages 2", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
		mg.Shutdown()
		assert.Equal(t, 0, httpmock.GetTotalCallCount())
	})
	t.Run("should return error when try to start twice", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		err := mg.Start()
		assert.Error(t, err)
		mg.Shutdown()
	})
	t.Run("should report if shutdown was done", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		ok := mg.Shutdown()
		assert.True(t, ok)
		ok = mg.Shutdown()
		assert.False(t, ok)
	})
	t.Run("can restart after shutdown", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		httpmock.RegisterResponder("POST", "https://www.example.com", httpmock.NewStringResponder(204, ""))
		mg := messenger.NewMessenger(c, q, "dummy", "https://www.example.com", st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		assert.True(t, mg.Shutdown())
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		item := &gofeed.Item{Content: "content", PublishedParsed: &now}
		if err := mg.AddMessage("dummy", feed, item, false); err != nil {
			t.Fatal(err)
		}
		for start := time.Now(); !q.IsEmpty(); {
			if time.Since(start) > 2*time.Second {
				t.Fatal("timed out waiting for message to be sent after restart")
			}
			time.Sleep(10 * time.Millisecond)
		}
		assert.True(t, mg.Shutdown())
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
	})
	t.Run("should shut down promptly while rate limited", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		const url = "https://www.example.com/limited"
		httpmock.RegisterResponder(
			"POST",
			url,
			httpmock.NewStringResponder(429, `{"retry_after": 3600}`).HeaderSet(http.Header{"Retry-After": {"3600"}}),
		)
		// own client, so the rate limit state does not leak into other tests
		mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", url, st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		first := &gofeed.Item{Title: "first-item", Content: "content", PublishedParsed: &now}
		if err := mg.AddMessage("dummy", feed, first, false); err != nil {
			t.Fatal(err)
		}
		for start := time.Now(); httpmock.GetTotalCallCount() == 0; {
			if time.Since(start) > 2*time.Second {
				t.Fatal("timed out waiting for POST")
			}
			time.Sleep(10 * time.Millisecond)
		}
		second := &gofeed.Item{Title: "second-item", Content: "content", PublishedParsed: &now}
		if err := mg.AddMessage("dummy", feed, second, false); err != nil {
			t.Fatal(err)
		}
		done := make(chan bool)
		go func() {
			done <- mg.Shutdown()
		}()
		select {
		case ok := <-done:
			assert.True(t, ok)
		case <-time.After(2 * time.Second):
			t.Fatal("shutdown did not return in time")
		}
		assert.Equal(t, 2, q.Size())
		v, err := q.Get(context.Background())
		require.NoError(t, err)
		assert.True(t, bytes.Contains(v, []byte("first-item")), "unsent message is no longer first")
		q.Clear()
	})
	t.Run("should not count shutdown while waiting for rate limit as error", func(t *testing.T) {
		st.ClearWebhookStats()
		q.Clear()
		httpmock.Reset()
		const url = "https://www.example.com/waiting"
		httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(204, ""))
		// allows one request only, so the second message waits for the limiter
		c := dhook.NewClient(dhook.WithWebhookRateLimit(1, time.Hour))
		mg := messenger.NewMessenger(c, q, "dummy", url, st, config.Config{})
		if err := mg.Start(); err != nil {
			t.Fatal(err)
		}
		feed := &gofeed.Feed{Title: "title"}
		now := time.Now()
		for _, title := range []string{"first-item", "second-item"} {
			item := &gofeed.Item{Title: title, Content: "content", PublishedParsed: &now}
			if err := mg.AddMessage("dummy", feed, item, false); err != nil {
				t.Fatal(err)
			}
		}
		for start := time.Now(); q.Size() != 1; {
			if time.Since(start) > 2*time.Second {
				t.Fatal("timed out waiting for first message to be sent")
			}
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(50 * time.Millisecond) // let the messenger block in the limiter
		mg.Shutdown()
		assert.Equal(t, 0, mg.Status().ErrorCount)
		assert.Equal(t, 1, httpmock.GetTotalCallCount())
		v, err := q.Get(context.Background())
		require.NoError(t, err)
		assert.True(t, bytes.Contains(v, []byte("second-item")))
		q.Clear()
	})
	for _, status := range []int{400, 413} {
		t.Run(fmt.Sprintf("should discard message on status %d without counting it as sent", status), func(t *testing.T) {
			st.ClearWebhookStats()
			q.Clear()
			httpmock.Reset()
			const url = "https://www.example.com/rejected"
			httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(status, `{"message": "rejected"}`))
			mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", url, st, config.Config{})
			if err := mg.Start(); err != nil {
				t.Fatal(err)
			}
			feed := &gofeed.Feed{Title: "title"}
			now := time.Now()
			item := &gofeed.Item{Content: "content", PublishedParsed: &now}
			if err := mg.AddMessage("dummy", feed, item, false); err != nil {
				t.Fatal(err)
			}
			for start := time.Now(); !q.IsEmpty(); {
				if time.Since(start) > 2*time.Second {
					t.Fatal("timed out waiting for message to be discarded")
				}
				time.Sleep(10 * time.Millisecond)
			}
			mg.Shutdown()
			assert.Equal(t, 1, httpmock.GetTotalCallCount())
			ws, err := st.GetWebhookStats("dummy")
			if err == nil {
				assert.Equal(t, 0, ws.SentCount)
			} else {
				assert.ErrorIs(t, err, storage.ErrNotFound)
			}
		})
	}
	for _, status := range []int{500, 401, 403, 404} {
		t.Run(fmt.Sprintf("should retry message on status %d", status), func(t *testing.T) {
			st.ClearWebhookStats()
			q.Clear()
			httpmock.Reset()
			const url = "https://www.example.com/flaky"
			httpmock.RegisterResponder("POST", url, httpmock.ResponderFromMultipleResponses([]*http.Response{
				httpmock.NewStringResponse(status, ""),
				httpmock.NewStringResponse(204, ""),
			}))
			mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", url, st, config.Config{})
			if err := mg.Start(); err != nil {
				t.Fatal(err)
			}
			feed := &gofeed.Feed{Title: "title"}
			now := time.Now()
			item := &gofeed.Item{Content: "content", PublishedParsed: &now}
			if err := mg.AddMessage("dummy", feed, item, false); err != nil {
				t.Fatal(err)
			}
			for start := time.Now(); !q.IsEmpty(); {
				if time.Since(start) > 2*time.Second {
					t.Fatal("timed out waiting for message to be sent")
				}
				time.Sleep(10 * time.Millisecond)
			}
			mg.Shutdown()
			assert.Equal(t, 2, httpmock.GetTotalCallCount())
			assert.Equal(t, 1, mg.Status().ErrorCount)
			ws, err := st.GetWebhookStats("dummy")
			require.NoError(t, err)
			assert.Equal(t, 1, ws.SentCount)
		})
	}
}
