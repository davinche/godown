package server

import (
	"fmt"
	"io"
	"net/http"

	"golang.org/x/net/websocket"
)

type renderString struct {
	Render string `json:"render"`
}

// Server is a websocket server
type Server struct {
	clients     map[int]*wsClient
	renderCh    chan string
	doneCh      chan struct{}
	HasShutdown chan struct{}
	prefix      string
}

type wsClient struct {
	id int
	ws *websocket.Conn
}

func (s *Server) handleConn(ws *websocket.Conn) {
	c := s.addClient(ws)
	for {
		var msg string
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			if err == io.EOF {
				s.delClient(c)
			} else {
				fmt.Printf("error reading from websocket: %q\n", err)
			}
		}
	}
}

// Listen begins listening for new markdown strings to broadcast
func (s *Server) Listen() {
	fmt.Println("Markdown Websocket Server Listening...")
	http.Handle(s.prefix, websocket.Handler(s.handleConn))

	// loop check for new strings to push to clients
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
	for id, c := range s.clients {
		err := websocket.JSON.Send(c.ws, rendered)
		if err != nil {
			fmt.Printf("error sending data: %q\n", err)
			delete(s.clients, id)
		}
	}
}

func (s *Server) addClient(ws *websocket.Conn) *wsClient {
	client := &wsClient{
		id: len(s.clients),
		ws: ws,
	}
	s.clients[client.id] = client
	return client
}

func (s *Server) delClient(c *wsClient) {
	delete(s.clients, c.id)
}

func (s *Server) close() {
	fmt.Println("Shutting Down...")
	for _, c := range s.clients {
		c.ws.Close()
	}
	s.HasShutdown <- struct{}{}
}

// NewServer creates a new websocket server that listens to client requests
func NewServer(prefix string) *Server {
	return &Server{
		prefix:      prefix,
		HasShutdown: make(chan struct{}),
		clients:     make(map[int]*wsClient),
		renderCh:    make(chan string),
		doneCh:      make(chan struct{}),
	}
}
