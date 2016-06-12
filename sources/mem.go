package sources

import (
	"crypto/sha1"
	"fmt"
	"log"
	"reflect"

	"github.com/davinche/godown/dispatch"
	"github.com/davinche/godown/server"
	"github.com/russross/blackfriday"
	"golang.org/x/net/websocket"
)

// MemRequest is the struct that represents the new in memory markdown file
type MemRequest struct {
	ID   string
	Data string
}

// Mem is used to track clients to in-memory markdown files
type Mem struct {
	dispatcher *dispatch.Dispatcher
	watching   map[string]map[*websocket.Conn]struct{}
	memData    map[string]string
	done       chan struct{}
}

// GetID returns a new unique identifer for a given string
func (m *Mem) GetID(id string) (string, error) {
	uid := getID(id)
	if _, ok := m.memData[uid]; ok {
		return uid, nil
	}
	return "", fmt.Errorf("memory warning: could not find tracked file: id=%q; uid=%q", id, uid)
}

// NewMem is the constructor for the Mem tracker
func NewMem(d *dispatch.Dispatcher) *Mem {
	return &Mem{
		dispatcher: d,
		watching:   make(map[string]map[*websocket.Conn]struct{}),
		memData:    make(map[string]string),
		done:       make(chan struct{}),
	}
}

// ServeRequest handles incoming messages from the dispatcher
func (m *Mem) ServeRequest(r *dispatch.Request) error {
	switch r.Type {
	case "MEM_ADD":
		return m.addFile(r.Value)
	case "FILE_DELETE":
		return m.delFile(r.Value.(string))
	case "ADD_WSCLIENT":
		return m.addClient(r.Value.(*server.WebsocketRequest))
	case "SHUTDOWN":
		return m.close()
	}
	return nil
}

// Wait is a blocking call until the Mem is shutdown
func (m *Mem) Wait() {
	<-m.done
}

func (m *Mem) addFile(r interface{}) error {
	v := reflect.ValueOf(r)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	vID := v.FieldByName("ID")
	vData := v.FieldByName("Data")

	if vID.Kind() == reflect.Invalid || vData.Kind() == reflect.Invalid {
		return fmt.Errorf("memory error: did not receive ID or Data: id=%v; data=;%v", vID, vData)
	}

	// get the id
	id := vID.String()

	// extract the bytes slice
	data := vData.Bytes()

	// markdownify
	mData := string(blackfriday.MarkdownCommon(data))

	uniqueID := getID(id)
	if _, ok := m.watching[uniqueID]; !ok {
		log.Printf("memory status: now accepting clients: id=%q\n", uniqueID)
		m.watching[uniqueID] = make(map[*websocket.Conn]struct{})
	}

	if _, ok := m.memData[uniqueID]; !ok {
		log.Printf("memory status: now tracking file: id=%q\n", uniqueID)
		m.memData[uniqueID] = mData
	}

	for client := range m.watching[uniqueID] {
		websocket.JSON.Send(client, RenderFormat{
			Render: mData,
		})
	}
	return nil
}

func (m *Mem) delFile(id string) error {
	uniqueID := getID(id)
	if watching, ok := m.watching[uniqueID]; ok {
		log.Printf("memory status: untracking file: id=%q\n", uniqueID)
		for client := range watching {
			client.Close()
		}
		delete(m.watching, uniqueID)
	}

	delete(m.memData, uniqueID)
	return nil
}

func (m *Mem) addClient(r *server.WebsocketRequest) error {
	mdata, ok := m.memData[r.ID]
	if !ok {
		log.Printf("memory error: could not find memory file to retrieve: id=%q\n", r.ID)
		return nil
	}

	watching := m.watching[r.ID]
	if _, ok := watching[r.WS]; !ok {
		log.Printf("memory status: adding client to the watch list: id=%q\n", r.ID)
		watching[r.WS] = struct{}{}
	}

	log.Printf("memory status: updating client with the markdown data: id=%q\n", r.ID)
	websocket.JSON.Send(r.WS, RenderFormat{
		Render: mdata,
	})
	return nil
}

func (m *Mem) getID(id string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(id)))
}

func (m *Mem) close() error {
	for _, watcher := range m.watching {
		for client := range watcher {
			client.Close()
		}
	}

	close(m.done)
	return nil
}
