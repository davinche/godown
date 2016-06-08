package subscribe

import (
	"sync"

	"golang.org/x/net/websocket"
)

// Handler is the interface for a weboscket subscriber
type Handler interface {
	Broadcast()
	Add(ws *websocket.Conn)
	Del(ws *websocket.Conn)
	Close()
}

// Source is where the struct that the server uses to identify the markdown resource
type Source interface {
	GetID() string
}

// ConnHandler tracks websocket connections
type ConnHandler struct {
	clients map[*websocket.Conn]struct{}
	sync.Mutex
}

// Add tracks a websocket connection against a watched file
func (c *ConnHandler) Add(ws *websocket.Conn) {
	c.Lock()
	defer c.Unlock()
	c.clients[ws] = struct{}{}
}

// Del removes a websocket connection against a watched file
func (c *ConnHandler) Del(ws *websocket.Conn) {
	c.Lock()
	defer c.Unlock()
	delete(c.clients, ws)
}

// Close closes all active websocket connections
func (c *ConnHandler) Close() {
	c.Lock()
	defer c.Unlock()
	for c := range c.clients {
		c.Close()
	}
}

// RenderFormat is the struct that holds the rendered markdown
type RenderFormat struct {
	Render string `json:"render"`
}
