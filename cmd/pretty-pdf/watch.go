package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	prettypdf "github.com/sazardev/go-pretty-pdf"
	"github.com/sazardev/go-pretty-pdf/cmd/pretty-pdf/output"
)

func runWatch(cmd *cobra.Command, args []string) error {
	if noColor {
		output.NoColor()
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}

	if !quiet {
		fmt.Println("  " + output.KeyValue("Watching", cfg.Source))
		fmt.Println("  " + output.KeyValue("Output", cfg.Output))
	}

	chromeExecPath, err := resolveChromePath()
	if err != nil {
		return fmt.Errorf("resolving Chrome: %w", err)
	}

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
		return fmt.Errorf("walking source: %w", err)
	}

	if !quiet {
		output.PrintWatchBanner()
	}

	var statsMu sync.Mutex
	stats := output.WatchStats{Running: true}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	rebuildCh := make(chan string, 1)

	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
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
					debounceTimer.Reset(300 * time.Millisecond)
				}
			case <-debounceTimer.C:
				rebuildCh <- "rebuild"
			}
		}
	}()

	go func() {
		for range sigCh {
			fmt.Println()
			statsMu.Lock()
			snapshot := stats
			statsMu.Unlock()
			output.PrintWatchSummary(snapshot)
			os.Exit(0)
		}
	}()

	// Kick off an initial build so the output exists immediately, not only
	// after the first file change — the whole point of `watch` is a
	// continuously current PDF.
	rebuildCh <- "rebuild"

	for range rebuildCh {
		output.PrintWatchRebuild()

		startTime := time.Now()

		opts := buildOpts(cfg, chromeExecPath, "")
		pdf, err := prettypdf.New(opts...)

		if err != nil {
			statsMu.Lock()
			stats.RecordError()
			statsMu.Unlock()
			fmt.Println(output.Error(fmt.Sprintf("Initialization failed: %v", err)))
			continue
		}

		spinner := output.StartSpinner("Rebuilding...")
		err = pdf.Build(cmd.Context())
		statsMu.Lock()
		if err != nil {
			stats.RecordError()
		} else {
			stats.RecordBuild()
		}
		statsMu.Unlock()
		if err != nil {
			spinner.Fail(err.Error())
		} else {
			spinner.Done(fmt.Sprintf("Done in %s", time.Since(startTime).Round(time.Millisecond)))
		}
	}

	return nil
}
