package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/russross/blackfriday"
	"golang.org/x/net/websocket"
)

var (
	maxClients int
)

type renderString struct {
	Render string `json:"render"`
}

// Server is a websocket server
type Server struct {
	clients map[int]*wsClient
	sync.Mutex
	renderCh    chan string
	doneCh      chan struct{}
	HasShutdown chan struct{}
	prefix      string
	file        string
}

type wsClient struct {
	id int
	ws *websocket.Conn
}

func getRenderedFromFile(file string) (string, error) {
	fileStr, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(blackfriday.MarkdownCommon(fileStr)), nil
}

func (s *Server) handleConn(ws *websocket.Conn) {
	c := s.addClient(ws)
	for {
		var msg string
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			s.delClient(c)
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
func (s *Server) Broadcast() {
	rendered, err := getRenderedFromFile(s.file)
	if err != nil {
		fmt.Printf("error reading file: %q", err)
		return
	}
	s.renderCh <- rendered
}

// Done proceeds to signal the shutdown of the websocket server
func (s *Server) Done() {
	s.doneCh <- struct{}{}
}

func (s *Server) broadcast(renderedStr string) {
	for _, c := range s.clients {
		err := s.sendToClient(c, renderedStr)
		if err != nil {
			fmt.Printf("error sending data: %q\n", err)
			s.delClient(c)
		}
	}
}

func (s *Server) sendToClient(c *wsClient, msg string) error {
	rendered := renderString{msg}
	return websocket.JSON.Send(c.ws, rendered)
}

func (s *Server) addClient(ws *websocket.Conn) *wsClient {
	s.Lock()
	// create client
	client := &wsClient{
		id: maxClients,
		ws: ws,
	}
	s.clients[client.id] = client
	maxClients++
	s.Unlock()

	// Send rendered to client
	rendered, err := getRenderedFromFile(s.file)
	if err == nil {
		s.sendToClient(client, rendered)
	}
	return client
}

func (s *Server) delClient(c *wsClient) {
	s.Lock()
	delete(s.clients, c.id)
	s.Unlock()
}

func (s *Server) close() {
	fmt.Println("Shutting Down...")
	for _, c := range s.clients {
		c.ws.Close()
	}
	s.HasShutdown <- struct{}{}
}

// NewServer creates a new websocket server that listens to client requests
func NewServer(prefix, file string) *Server {
	return &Server{
		prefix:      prefix,
		file:        file,
		HasShutdown: make(chan struct{}),
		clients:     make(map[int]*wsClient),
		renderCh:    make(chan string),
		doneCh:      make(chan struct{}),
	}
}
