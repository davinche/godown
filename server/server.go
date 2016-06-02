package server

import (
	"fmt"
	"net/http"

	"golang.org/x/net/websocket"
)

type renderString struct {
	Render string `json:"render"`
}

// Server is a websocket server
type Server struct {
	clients     []*websocket.Conn
	renderCh    chan string
	doneCh      chan struct{}
	Prefix      string
	HasShutdown chan struct{}
}

func (s *Server) handleConn(ws *websocket.Conn) {
	s.addClient(ws)
}

// Listen begins listening for new markdown strings to broadcast
func (s *Server) Listen() {
	fmt.Println("Markdown Websocket Server Listening...")
	http.Handle(s.Prefix, websocket.Handler(s.handleConn))

	for {
		select {
		case renderedStr := <-s.renderCh:
			s.broadcast(renderedStr)
		case <-s.doneCh:
			s.close()
			return
		}
	}
}

// Broadcast sends the Markdown rendered updates to all websocket clients
func (s *Server) Broadcast(renderedStr string) {
	s.renderCh <- renderedStr
}

// Done proceeds to signal the shutdown of the websocket server
func (s *Server) Done() {
	s.doneCh <- struct{}{}
}

func (s *Server) broadcast(renderedStr string) {
	rendered := renderString{renderedStr}
	for _, c := range s.clients {
		websocket.JSON.Send(c, rendered)
	}
}

func (s *Server) addClient(ws *websocket.Conn) {
	s.clients = append(s.clients, ws)
}

func (s *Server) close() {
	fmt.Println("Shutting Down...")
	for _, c := range s.clients {
		c.Close()
	}
	s.HasShutdown <- struct{}{}
}

// NewServer creates a new websocket server that listens to client requests
func NewServer(prefix string) *Server {
	return &Server{
		Prefix:      prefix,
		HasShutdown: make(chan struct{}),
		clients:     make([]*websocket.Conn, 0),
		renderCh:    make(chan string),
		doneCh:      make(chan struct{}),
	}
}
