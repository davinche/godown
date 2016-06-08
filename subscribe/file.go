package subscribe

import (
	"fmt"
	"io/ioutil"

	"github.com/davinche/godown/markdown"
	"github.com/russross/blackfriday"
	"golang.org/x/net/websocket"
)

// ----------------------------------------------------------------------------
// Utility
// ----------------------------------------------------------------------------
func getRenderedFromFile(file string) (string, error) {
	fileStr, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(blackfriday.MarkdownCommon(fileStr)), nil
}

// NewFile creates a new file changes writer
func NewFile(f *markdown.File) *File {
	return &File{
		file: f,
		ConnHandler: ConnHandler{
			clients: make(map[*websocket.Conn]struct{}),
		},
	}
}

// File tracks all subscribers to a file and writes the new changes to them
// on file change
type File struct {
	file *markdown.File
	ConnHandler
}

// Broadcast sends all registered websockets with the updated file
func (f *File) Broadcast() {
	fileStr, err := getRenderedFromFile(f.file.Path)
	if err != nil {
		fmt.Printf("error reading file: %q", err)
		return
	}
	renderFmt := RenderFormat{fileStr}
	// loop and write
	f.Lock()
	defer f.Unlock()
	for ws := range f.clients {
		websocket.JSON.Send(ws, renderFmt)
	}
}

// Add tracks a websocket connection against a watched file
func (f *File) Add(ws *websocket.Conn) {
	f.ConnHandler.Add(ws)
	fileStr, err := getRenderedFromFile(f.file.Path)
	if err != nil {
		fmt.Printf("error reading file: %q", err)
		return
	}
	renderFmt := RenderFormat{fileStr}
	websocket.JSON.Send(ws, renderFmt)
}
