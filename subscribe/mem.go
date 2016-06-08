package subscribe

import (
	"github.com/davinche/godown/memory"
	"github.com/russross/blackfriday"
	"golang.org/x/net/websocket"
)

// NewMem creates a new in memory changes tracker
func NewMem(f *memory.File) *Mem {
	return &Mem{
		file: f,
		ConnHandler: ConnHandler{
			clients: make(map[*websocket.Conn]struct{}),
		},
	}
}

// Mem tracks all subscribers to an in-memory markdown file
type Mem struct {
	file *memory.File
	ConnHandler
}

// Broadcast sends all subscribed websockets with new file changes
func (m *Mem) Broadcast() {
	rendered := string(blackfriday.MarkdownCommon(m.file.Content))
	renderFmt := RenderFormat{rendered}
	// loop and write
	m.Lock()
	defer m.Unlock()
	for ws := range m.clients {
		websocket.JSON.Send(ws, renderFmt)
	}
}

// Add tracks a websocket connection against the in memory markdown
func (m *Mem) Add(ws *websocket.Conn) {
	m.ConnHandler.Add(ws)
	rendered := string(blackfriday.MarkdownCommon(m.file.Content))
	renderFmt := RenderFormat{rendered}
	websocket.JSON.Send(ws, renderFmt)
}

// Update replaces the current in-memory file with a new one
func (m *Mem) Update(f *memory.File) {
	m.file = f
}
