package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/davinche/godown/markdown"
	"github.com/davinche/godown/memory"
	"github.com/davinche/godown/subscribe"
	"golang.org/x/net/websocket"
)

// Server is a websocket server
type Server struct {
	prefix string

	catalog map[string]subscribe.Handler
	sync.Mutex
}

// CreateSubscriber creates a new registry for a file and it's subscribed clients
func (s *Server) CreateSubscriber(source subscribe.Source) {
	s.Lock()
	defer s.Unlock()
	switch source.(type) {
	case *markdown.File:
		if _, ok := s.catalog[source.GetID()]; !ok {
			s.catalog[source.GetID()] = subscribe.NewFile(source.(*markdown.File))
		}
	case *memory.File:
		if c, ok := s.catalog[source.GetID()]; !ok {
			s.catalog[source.GetID()] = subscribe.NewMem(source.(*memory.File))
		} else {
			// try to assert memory tracker
			if m, ok := c.(*subscribe.Mem); ok {
				m.Update(source.(*memory.File))
				m.Broadcast()
			}
		}
	}
}

// RemoveSubscriber removes the catalog of subscribers from being tracked
func (s *Server) RemoveSubscriber(id string) {
	s.Lock()
	defer s.Unlock()
	if c, ok := s.catalog[id]; ok {
		c.Close()
		delete(s.catalog, id)
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
	s.catalog[fileID].Add(ws)
	return true
}

func (s *Server) deleteClient(fileID string, ws *websocket.Conn) {
	s.Lock()
	defer s.Unlock()
	if s.catalog[fileID] == nil {
		return
	}
	s.catalog[fileID].Del(ws)
}

// Broadcast to all subscribers of fileID the new file
func (s *Server) Broadcast(file subscribe.Source) {
	fSubs := s.catalog[file.GetID()]
	if fSubs != nil {
		fSubs.Broadcast()
	}
}

// NewServer creates a new websocket server that listens to client requests
func NewServer() *Server {
	return &Server{
		catalog: make(map[string]subscribe.Handler),
	}
}
