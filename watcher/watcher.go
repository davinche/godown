package watcher

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/davinche/godown/markdown"
)

// WatchFileRequest is the format of the http request type
type WatchFileRequest struct {
	File string `json:"file"`
}

type watchFile struct {
	done chan struct{}
	file *markdown.File
}

func (w *watchFile) Done() {
	w.done <- struct{}{}
}

func (w *watchFile) Start(report chan *markdown.File) error {
	stat, err := os.Stat(w.file.Path)
	if err != nil {
		return fmt.Errorf("error: could not read file: %q\n", err)
	}
	go func() {
		for {
			select {
			case <-w.done:
				return
			case <-time.After(time.Second):
				newStat, err := os.Stat(w.file.Path)
				if err != nil {
					continue
				}
				// something changed, broadcast rerender to all clients
				if newStat.Size() != stat.Size() || newStat.ModTime() != stat.ModTime() {
					report <- w.file
					stat = newStat
				}
			}
		}
	}()
	return nil
}

// WatchMonitor is a monitor for all watched files
type WatchMonitor struct {
	done    chan struct{}
	changes chan *markdown.File
	sync.Mutex
	watchers map[string]*watchFile
}

// NewWatchMonitor is the constructor for a WatchMonitor
func NewWatchMonitor() *WatchMonitor {
	mon := WatchMonitor{
		done:     make(chan struct{}),
		watchers: make(map[string]*watchFile),
		changes:  make(chan *markdown.File),
	}
	return &mon
}

// AddWatcher creates a new watcher for a particular file
func (w *WatchMonitor) AddWatcher(f *markdown.File) {
	w.Lock()
	defer w.Unlock()
	if w.watchers[f.GetID()] != nil {
		return
	}
	watcher := &watchFile{
		file: f,
		done: make(chan struct{}),
	}
	w.watchers[f.GetID()] = watcher
	go watcher.Start(w.changes)
}

// RemoveWatcher deletes a file from being watched
func (w *WatchMonitor) RemoveWatcher(fileID string) {
	w.Lock()
	defer w.Unlock()
	if w.watchers[fileID] == nil {
		return
	}
	watcher := w.watchers[fileID]
	watcher.Done()
	delete(w.watchers, fileID)
}

// Done stops watching all files
func (w *WatchMonitor) close() {
	w.Lock()
	defer w.Unlock()
	for _, watcher := range w.watchers {
		watcher.Done()
	}
}

// Done stops watching all files
func (w *WatchMonitor) Done() {
	w.done <- struct{}{}
}

// Start begins the watching/reporting of file changes
func (w *WatchMonitor) Start(report chan *markdown.File) {
	for {
		select {
		case <-w.done:
			w.close()
		case f := <-w.changes:
			report <- f
		}
	}
}
