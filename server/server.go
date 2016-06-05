package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/davinche/godown/markdown"
	"github.com/russross/blackfriday"
	"golang.org/x/net/websocket"
)

// ----------------------------------------------------------------------------
// Utility
// ----------------------------------------------------------------------------
type renderString struct {
	Render string `json:"render"`
}

func getRenderedFromFile(file string) (string, error) {
	fileStr, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(blackfriday.MarkdownCommon(fileStr)), nil
}

// ----------------------------------------------------------------------------
// Data
// ----------------------------------------------------------------------------

// track subscribers to a file
type fileSubscribers struct {
	file *markdown.File

	clients map[*websocket.Conn]struct{}
	sync.Mutex
}

// broadcast all changes to subscribers
func (f *fileSubscribers) broadcast() {
	fileStr, err := getRenderedFromFile(f.file.Path)
	if err != nil {
		fmt.Printf("error reading file: %q", err)
		return
	}
	renderFmt := renderString{fileStr}
	// loop and write
	f.Lock()
	defer f.Unlock()
	for ws := range f.clients {
		websocket.JSON.Send(ws, renderFmt)
	}
}

func (f *fileSubscribers) add(ws *websocket.Conn) {
	f.Lock()
	defer f.Unlock()
	f.clients[ws] = struct{}{}
	fileStr, err := getRenderedFromFile(f.file.Path)
	if err != nil {
		fmt.Printf("error reading file: %q", err)
		return
	}
	renderFmt := renderString{fileStr}
	websocket.JSON.Send(ws, renderFmt)
}

func (f *fileSubscribers) del(ws *websocket.Conn) (shouldDelete bool) {
	f.Lock()
	defer f.Unlock()
	delete(f.clients, ws)
	if len(f.clients) == 0 {
		return true
	}
	return false
}

// Server is a websocket server
type Server struct {
	prefix string

	catalog map[string]*fileSubscribers
	sync.Mutex
}

// CreateFileSubscriber creates a new registry for a file and it's subscribed clients
func (s *Server) CreateFileSubscriber(file *markdown.File) {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.catalog[file.ID]; !ok {
		s.catalog[file.ID] = &fileSubscribers{
			file:    file,
			clients: make(map[*websocket.Conn]struct{}),
		}
	}
}

// Register begins listening for new markdown strings to broadcast
func (s *Server) Register(prefix string) {
	fmt.Println("Markdown Websocket Server Listening...")
	// http.Handle(s.prefix, websocket.Handler(s.handleConn))
	http.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		// Make sure a file id is specified
		fileID := r.URL.Query().Get("id")
		if fileID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// create the websocket handler
		handleWS := func(ws *websocket.Conn) {
			hasAdded := s.addClient(fileID, ws)
			if hasAdded {
				for {
					var msg string
					err := websocket.Message.Receive(ws, &msg)
					if err != nil {
						s.deleteClient(fileID, ws)
						return
					}
				}
			}
		}
		websocket.Handler(handleWS).ServeHTTP(w, r)
	})
}

func (s *Server) addClient(fileID string, ws *websocket.Conn) (success bool) {
	s.Lock()
	defer s.Unlock()
	if s.catalog[fileID] == nil {
		return false
	}
	s.catalog[fileID].add(ws)
	return true
}

func (s *Server) deleteClient(fileID string, ws *websocket.Conn) {
	s.Lock()
	defer s.Unlock()
	if s.catalog[fileID] == nil {
		return
	}
}

// Broadcast to all subscribers of fileID the new file
func (s *Server) Broadcast(file *markdown.File) {
	fSubs := s.catalog[file.ID]
	if fSubs != nil {
		fSubs.broadcast()
	}
}

// NewServer creates a new websocket server that listens to client requests
func NewServer(f *markdown.File) *Server {
	s := &Server{
		catalog: make(map[string]*fileSubscribers),
	}
	s.CreateFileSubscriber(f)
	return s
}
