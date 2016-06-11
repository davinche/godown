package sources

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/net/websocket"

	"github.com/davinche/godown/dispatch"
	"github.com/davinche/godown/server"
	"github.com/russross/blackfriday"
)

// File is used to track watched files
type File struct {
	dispatcher *dispatch.Dispatcher
	watching   map[string]map[*websocket.Conn]struct{}
	watchers   map[string]*Watcher
	done       chan struct{}
}

// NewFile is the constructor for a new Files tracker
func NewFile(d *dispatch.Dispatcher) *File {
	return &File{
		dispatcher: d,
		watching:   make(map[string]map[*websocket.Conn]struct{}),
		watchers:   make(map[string]*Watcher),
		done:       make(chan struct{}),
	}
}

// ServeRequest handles dispatched messages
func (f *File) ServeRequest(r *dispatch.Request) error {
	switch r.Type {
	case "FILE_ADD":
		return f.addFile(r.Value.(string))
	case "FILE_DELETE":
		return f.delFile(r.Value.(string))
	case "FILE_CHANGE":
		change := r.Value.(*fileChange)
		return f.broadcast(change)
	case "ADD_WSCLIENT":
		clientRequest := r.Value.(*server.WebsocketRequest)
		return f.addClient(clientRequest)
	case "SHUTDOWN":
		return f.close()
	}
	return nil
}

// Wait for termination
func (f *File) Wait() {
	<-f.done
}

// GetID returns a unique id for a given file
func (f *File) GetID(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("file error: cannot get absolute path: err=%q\n", err)
	}
	return getID(absPath), nil
}

// adds a file to be watched
func (f *File) addFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Printf("file error: cannot get absolute path: err=%q\n", err)
		return nil
	}
	id := getID(absPath)
	if _, ok := f.watching[id]; !ok {
		log.Printf("file status: added new connection list for file: id=%q\n", id)
		f.watching[id] = make(map[*websocket.Conn]struct{})
	}

	if _, ok := f.watchers[id]; !ok {
		log.Printf("file status: started watching file: id=%q\n", id)
		watcher := NewWatcher(f.dispatcher, absPath)
		f.watchers[id] = watcher
		watcher.Start()
	}
	return nil
}

func (f *File) addClient(request *server.WebsocketRequest) error {
	// see if we're already watching the file
	watching, ok := f.watching[request.ID]
	if !ok {
		log.Printf("watching error: currently not watching file: id=%q\n", request.ID)
		return nil
	}

	// Add the client to the set of file listeners
	_, ok = watching[request.WS]
	if !ok {
		log.Printf("watching status: adding client to the watch list: id=%q\n", request.ID)
		watching[request.WS] = struct{}{}
	}

	// Ask our watcher to update the client
	if watcher, ok := f.watchers[request.ID]; ok {
		log.Printf("watching status: updating client with new data: id=%q\n", request.ID)
		watcher.Update(request.WS)
	}
	return nil
}

// deletes a file from being watched
func (f *File) delFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Printf("file error: cannot get absolute path: err=%q\n", err)
		return nil
	}

	id := getID(absPath)
	// close the currently opened websockets
	if watching, ok := f.watching[id]; ok {
		log.Printf("file status: untracking file: id=%q\n", id)
		for client := range watching {
			client.Close()
		}
		delete(f.watching, id)
	}

	// stop watching the file
	if watcher, ok := f.watchers[id]; ok {
		watcher.Close()
		delete(f.watchers, id)
	}
	return nil
}

func (f *File) broadcast(change *fileChange) error {
	id := getID(change.Path)
	if watching, ok := f.watching[id]; ok {
		for client := range watching {
			err := websocket.JSON.Send(client, RenderFormat{
				Render: change.Value,
			})
			if err != nil {
				delete(watching, client)
			}
		}
	}
	return nil
}

func (f *File) close() error {
	for _, watcher := range f.watching {
		for client := range watcher {
			client.Close()
		}
	}

	for _, watcher := range f.watchers {
		watcher.Close()
	}
	close(f.done)
	return nil
}

// ----------------------------------------------------------------------------
// Watcher --------------------------------------------------------------------
// ----------------------------------------------------------------------------

// NewWatcher is the constructor for a new file watcher
func NewWatcher(d *dispatch.Dispatcher, filePath string) *Watcher {
	return &Watcher{
		dispatcher: d,
		filePath:   filePath,
		done:       make(chan struct{}),
	}
}

// Watcher watches a file for file changes
type Watcher struct {
	dispatcher *dispatch.Dispatcher
	filePath   string
	done       chan struct{}
}

// Start begins watching a file
func (w *Watcher) Start() (string, error) {
	log.Printf("watcher status: starting watcher: file=%q", w.filePath)
	stat, err := os.Stat(w.filePath)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadFile(w.filePath)
	if err != nil {
		return "", err
	}

	go func() {
		for {
			select {
			case <-w.done:
				return
			case <-time.After(time.Second):
				newStat, err := os.Stat(w.filePath)
				if err != nil {
					continue
				}
				if newStat.Size() != stat.Size() || newStat.ModTime() != stat.ModTime() {
					log.Printf("watcher status: change detected: file=%q", w.filePath)
					data, err := ioutil.ReadFile(w.filePath)
					if err != nil {
						continue
					}
					w.dispatcher.Dispatch("FILE_CHANGE", &fileChange{
						Path:  w.filePath,
						Value: string(blackfriday.MarkdownCommon(data)),
					})
					stat = newStat
				}
			}
		}

	}()

	return string(blackfriday.MarkdownCommon(data)), nil
}

// Update sends the client the markdown data from our file
func (w *Watcher) Update(ws *websocket.Conn) {
	data, err := ioutil.ReadFile(w.filePath)
	if err != nil {
		return
	}
	websocket.JSON.Send(ws, RenderFormat{
		Render: string(blackfriday.MarkdownCommon(data)),
	})
}

// Close signals the watcher to stop watching the file
func (w *Watcher) Close() {
	close(w.done)
}

// ----------------------------------------------------------------------------
// HELPERS --------------------------------------------------------------------
// ----------------------------------------------------------------------------

// transport struct for reporting file changes
type fileChange struct {
	Path  string
	Value string
}

// heleper to create a unique id for a file path
func getID(path string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(path)))
}
