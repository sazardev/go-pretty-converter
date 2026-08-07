package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	prettypdf "github.com/sazardev/go-pretty-pdf"
	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
)

type liveServer struct {
	mu       sync.RWMutex
	html     string
	reloadCh chan struct{}
}

func runServe(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	output.PrintBanner("")
	fmt.Println("  " + output.KeyValue("Source", cfg.Source))
	fmt.Println("  " + output.KeyValue("Port", fmt.Sprintf("%d", servePort)))
	fmt.Println()

	// serve never calls pdf.Render/Build (it only parses+composes HTML for
	// live preview), so it needs no Chrome resolution at all.
	opts := buildOpts(cfg, "")
	pdf, err := prettypdf.New(opts...)
	if err != nil {
		return fmt.Errorf("initializing: %w", err)
	}

	ls := &liveServer{
		reloadCh: make(chan struct{}),
	}

	if rebuildErr := ls.rebuild(pdf); rebuildErr != nil {
		return rebuildErr
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", ls.serveHTML)
	mux.HandleFunc("/events", ls.serveSSE)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	if err := filepath.WalkDir(cfg.Source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return watcher.Add(path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("watching source: %w", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				ext := filepath.Ext(event.Name)
				if ext != ".mdx" && ext != ".md" && ext != ".txt" && ext != ".yaml" && ext != ".yml" {
					continue
				}
				if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
					fmt.Println("  " + output.Info("Change detected, recompiling..."))
					if err := ls.rebuild(pdf); err != nil {
						fmt.Println("  " + output.Error(fmt.Sprintf("Rebuild failed: %v", err)))
					} else {
						ls.notifyReload()
						fmt.Println("  " + output.Success("HTML updated"))
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("  " + output.Error(fmt.Sprintf("Watcher error: %v", err)))
			}
		}
	}()

	fmt.Println("  " + output.Success(fmt.Sprintf("Preview at http://localhost:%d", servePort)))
	fmt.Println("  " + output.MutedStyle.Render("Press Ctrl+C to stop"))
	fmt.Println()

	addr := fmt.Sprintf(":%d", servePort)
	if err := http.ListenAndServe(addr, mux); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (ls *liveServer) rebuild(pdf *prettypdf.PDF) error {
	docs, err := pdf.ParseDir()
	if err != nil && len(docs) == 0 {
		return fmt.Errorf("parsing: %w", err)
	}

	html, err := pdf.ComposeHTML(docs)
	if err != nil {
		return fmt.Errorf("composing HTML: %w", err)
	}

	html = injectLiveReload(html)

	ls.mu.Lock()
	ls.html = html
	ls.mu.Unlock()

	return nil
}

// notifyReload closes the current reload channel (waking any /events
// connections blocked on it) and replaces it with a fresh one. The whole
// close-and-replace happens under a single write lock rather than a
// separate read-then-write pair: two notifyReload calls racing on the
// latter would both read the same (not-yet-replaced) channel and the
// second close would panic on an already-closed channel.
func (ls *liveServer) notifyReload() {
	ls.mu.Lock()
	close(ls.reloadCh)
	ls.reloadCh = make(chan struct{})
	ls.mu.Unlock()
}

func (ls *liveServer) serveHTML(w http.ResponseWriter, r *http.Request) {
	ls.mu.RLock()
	html := ls.html
	ls.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

func (ls *liveServer) serveSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	_, _ = fmt.Fprintf(w, "event: connected\ndata:\n\n")
	flusher.Flush()

	ls.mu.RLock()
	ch := ls.reloadCh
	ls.mu.RUnlock()

	ctx := r.Context()
	select {
	case <-ch:
		_, _ = fmt.Fprintf(w, "event: reload\ndata:\n\n")
		flusher.Flush()
	case <-ctx.Done():
	}
}

func injectLiveReload(html string) string {
	script := `<script>
(function(){var e=new EventSource('/events');e.addEventListener('reload',function(){location.reload()})})();
</script>`
	return html + script
}
