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
	"testing/synctest"
	"time"

	bolt "go.etcd.io/bbolt"

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
	synctest.Test(t, func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "test.db")
		db, err := bolt.Open(p, 0600, nil)
		require.NoError(t, err)
		q, err := pqueue.New(db, "test")
		require.NoError(t, err)
		st := storage.New(db)
		db.Close() // every queue access fails from now on
		h := &countingHandler{prefix: "Failed to process queue item"}
		defer slog.SetDefault(slog.Default())
		slog.SetDefault(slog.New(h))
		mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, false)
		err = mg.Start()
		require.NoError(t, err)
		synctest.Wait()
		require.Equal(t, int64(1), h.count.Load())

		time.Sleep(10*time.Second - time.Millisecond)
		synctest.Wait()
		assert.Equal(t, int64(1), h.count.Load(), "retried too early")

		time.Sleep(time.Millisecond)
		synctest.Wait()
		assert.Equal(t, int64(2), h.count.Load(), "did not retry after delay")

		start := time.Now()
		assert.True(t, mg.Shutdown())
		assert.Zero(t, time.Since(start), "shutdown was not immediate")
	})
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
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	t.Run("can return name", func(t *testing.T) {
		// given
		st, q := newTestStorageAndQueue(t)
		// when
		mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, false)
		// then
		assert.Equal(t, "dummy", mg.Name())
	})
	t.Run("can submit messages", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// given
			st, q := newTestStorageAndQueue(t)
			httpmock.Reset()
			httpmock.RegisterResponder(
				"POST",
				"https://www.example.com",
				httpmock.NewStringResponder(204, ""),
			)
			mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, false)
			err := mg.Start()
			require.NoError(t, err)
			t.Cleanup(func() { mg.Shutdown() })
			feed := &gofeed.Feed{Title: "title"}
			now := time.Now()
			item := &gofeed.Item{Content: "content", PublishedParsed: &now}
			// when
			err = mg.AddMessage("dummy", feed, item, false)
			require.NoError(t, err)
			synctest.Wait()
			// then
			assert.Equal(t, 1, httpmock.GetTotalCallCount())
			ws, err := st.GetWebhookStats("dummy")
			require.NoError(t, err)
			assert.Equal(t, 1, ws.SentCount)
		})
	})
	t.Run("can submit messages 2", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			st, q := newTestStorageAndQueue(t)
			httpmock.Reset()
			mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, false)
			err := mg.Start()
			require.NoError(t, err)
			t.Cleanup(func() { mg.Shutdown() })
			synctest.Wait()
			assert.Equal(t, 0, httpmock.GetTotalCallCount())
		})
	})
	t.Run("should return error when try to start twice", func(t *testing.T) {
		st, q := newTestStorageAndQueue(t)
		httpmock.Reset()
		mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, false)
		err := mg.Start()
		require.NoError(t, err)
		t.Cleanup(func() { mg.Shutdown() })
		err = mg.Start()
		assert.Error(t, err)
	})
	t.Run("should report if shutdown was done", func(t *testing.T) {
		st, q := newTestStorageAndQueue(t)
		httpmock.Reset()
		mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, false)
		err := mg.Start()
		require.NoError(t, err)
		ok := mg.Shutdown()
		assert.True(t, ok)
		ok = mg.Shutdown()
		assert.False(t, ok)
	})
	t.Run("can restart after shutdown", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			st, q := newTestStorageAndQueue(t)
			httpmock.Reset()
			httpmock.RegisterResponder("POST", "https://www.example.com", httpmock.NewStringResponder(204, ""))
			mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", "https://www.example.com", st, false)
			err := mg.Start()
			require.NoError(t, err)
			assert.True(t, mg.Shutdown())
			err = mg.Start()
			require.NoError(t, err)
			feed := &gofeed.Feed{Title: "title"}
			now := time.Now()
			item := &gofeed.Item{Content: "content", PublishedParsed: &now}
			err = mg.AddMessage("dummy", feed, item, false)
			require.NoError(t, err)
			synctest.Wait()
			assert.True(t, q.IsEmpty(), "message was not sent after restart")
			assert.True(t, mg.Shutdown())
			assert.Equal(t, 1, httpmock.GetTotalCallCount())
		})
	})
	t.Run("should shut down promptly while rate limited", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			st, q := newTestStorageAndQueue(t)
			httpmock.Reset()
			const url = "https://www.example.com/limited"
			httpmock.RegisterResponder(
				"POST",
				url,
				httpmock.NewStringResponder(429, `{"retry_after": 3600}`).HeaderSet(http.Header{"Retry-After": {"3600"}}),
			)
			mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", url, st, false)
			err := mg.Start()
			require.NoError(t, err)
			feed := &gofeed.Feed{Title: "title"}
			now := time.Now()
			first := &gofeed.Item{Title: "first-item", Content: "content", PublishedParsed: &now}
			err = mg.AddMessage("dummy", feed, first, false)
			require.NoError(t, err)
			synctest.Wait()
			require.Equal(t, 1, httpmock.GetTotalCallCount())
			second := &gofeed.Item{Title: "second-item", Content: "content", PublishedParsed: &now}
			err = mg.AddMessage("dummy", feed, second, false)
			require.NoError(t, err)
			start := time.Now()
			assert.True(t, mg.Shutdown())
			assert.Zero(t, time.Since(start), "shutdown was not immediate")
			assert.Equal(t, 2, q.Size())
			v, err := q.Get(context.Background())
			require.NoError(t, err)
			assert.True(t, bytes.Contains(v, []byte("first-item")), "unsent message is no longer first")
		})
	})
	t.Run("should not count shutdown while waiting for rate limit as error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			st, q := newTestStorageAndQueue(t)
			httpmock.Reset()
			const url = "https://www.example.com/waiting"
			httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(204, ""))
			// allows one request only, so the second message waits for the limiter
			c := dhook.NewClient(dhook.WithWebhookRateLimit(1, time.Hour))
			mg := messenger.NewMessenger(c, q, "dummy", url, st, false)
			err := mg.Start()
			require.NoError(t, err)
			feed := &gofeed.Feed{Title: "title"}
			now := time.Now()
			for _, title := range []string{"first-item", "second-item"} {
				item := &gofeed.Item{Title: title, Content: "content", PublishedParsed: &now}
				err := mg.AddMessage("dummy", feed, item, false)
				require.NoError(t, err)
			}
			synctest.Wait() // first message sent, messenger blocks in the limiter
			require.Equal(t, 1, q.Size())
			mg.Shutdown()
			assert.Equal(t, 0, mg.Status().ErrorCount)
			assert.Equal(t, 1, httpmock.GetTotalCallCount())
			v, err := q.Get(context.Background())
			require.NoError(t, err)
			assert.True(t, bytes.Contains(v, []byte("second-item")))
		})
	})
	for _, status := range []int{400, 413} {
		t.Run(fmt.Sprintf("should discard message on status %d without counting it as sent", status), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				st, q := newTestStorageAndQueue(t)
				httpmock.Reset()
				const url = "https://www.example.com/rejected"
				httpmock.RegisterResponder("POST", url, httpmock.NewStringResponder(status, `{"message": "rejected"}`))
				mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", url, st, false)
				err := mg.Start()
				require.NoError(t, err)
				feed := &gofeed.Feed{Title: "title"}
				now := time.Now()
				item := &gofeed.Item{Content: "content", PublishedParsed: &now}
				err = mg.AddMessage("dummy", feed, item, false)
				require.NoError(t, err)
				synctest.Wait()
				assert.True(t, q.IsEmpty(), "message was not discarded")
				mg.Shutdown()
				assert.Equal(t, 1, httpmock.GetTotalCallCount())
				ws, err := st.GetWebhookStats("dummy")
				if err == nil {
					assert.Equal(t, 0, ws.SentCount)
				} else {
					assert.ErrorIs(t, err, storage.ErrNotFound)
				}
			})
		})
	}
	for _, status := range []int{500, 401, 403, 404} {
		t.Run(fmt.Sprintf("should retry message on status %d", status), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				st, q := newTestStorageAndQueue(t)
				httpmock.Reset()
				const url = "https://www.example.com/flaky"
				httpmock.RegisterResponder("POST", url, httpmock.ResponderFromMultipleResponses([]*http.Response{
					httpmock.NewStringResponse(status, ""),
					httpmock.NewStringResponse(204, ""),
				}))
				mg := messenger.NewMessenger(dhook.NewClient(), q, "dummy", url, st, false)
				err := mg.Start()
				require.NoError(t, err)
				feed := &gofeed.Feed{Title: "title"}
				now := time.Now()
				item := &gofeed.Item{Content: "content", PublishedParsed: &now}
				err = mg.AddMessage("dummy", feed, item, false)
				require.NoError(t, err)
				time.Sleep(time.Minute) // longer than the retry backoff
				assert.True(t, q.IsEmpty(), "message was not sent")
				mg.Shutdown()
				assert.Equal(t, 2, httpmock.GetTotalCallCount())
				assert.Equal(t, 1, mg.Status().ErrorCount)
				ws, err := st.GetWebhookStats("dummy")
				require.NoError(t, err)
				assert.Equal(t, 1, ws.SentCount)
			})
		})
	}
}

func newTestStorageAndQueue(t *testing.T) (*storage.Storage, *pqueue.PQueue) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(p, 0600, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})
	q, err := pqueue.New(db, "test")
	require.NoError(t, err)
	st := storage.New(db)
	err = st.Init(nil)
	require.NoError(t, err)
	return st, q
}
