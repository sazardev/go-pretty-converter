package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestLiveServerConcurrentReloadAndSSENoRace guards a real data race: an
// SSE client's serveSSE used to read ls.reloadCh with no lock while
// notifyReload concurrently closed and reassigned it under ls.mu — a data
// race flagged by `go test -race` that could make an in-flight /events
// connection observe a stale, already-closed channel and never see a
// subsequent reload notification. Run with `go test -race` to be
// meaningful; without it this only checks the code doesn't panic/deadlock.
//
// notifyReload itself is only ever called from the single fsnotify watcher
// goroutine in production (never concurrently with itself), so it's
// exercised here sequentially from one goroutine — same as real usage —
// while serveSSE/serveHTML run concurrently against it from many.
func TestLiveServerConcurrentReloadAndSSENoRace(t *testing.T) {
	ls := &liveServer{reloadCh: make(chan struct{}), html: "<html></html>"}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			ls.notifyReload()
		}
	}()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/events", nil)
			ctx, cancel := context.WithTimeout(req.Context(), 200*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			ls.serveSSE(rec, req)
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			ls.serveHTML(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		}()
	}

	wg.Wait()
}
