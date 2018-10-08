package coordinator

import (
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/davinche/godown/dispatch"
	"github.com/davinche/godown/server"
	"github.com/davinche/godown/sources"
)

// Coordinator orchestrates incoming requests
type Coordinator struct {
	listener net.Listener
	port     int
	done     chan struct{}
	sources  []sources.Source
}

// New is the constructor for request coordination
func New(port int) (*Coordinator, error) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}
	return &Coordinator{
		listener: listener,
		port:     port,
		done:     make(chan struct{}),
		sources:  make([]sources.Source, 0),
	}, nil
}

// Serve instantiates all the parts required to host the markdown daemon
func (c *Coordinator) Serve() {
	dispatcher := dispatch.NewDispatcher()
	apiServer := server.NewAPI(dispatcher)
	websocketServer := server.NewWebsocket(dispatcher)
	filesServer := server.NewStatic()

	// Sources of markdown
	fileSource := sources.NewFile(dispatcher)
	memSource := sources.NewMem(dispatcher)
	dispatcher.AddHandler(fileSource)
	dispatcher.AddHandler(memSource)
	dispatcher.AddHandler(filesServer)

	// Track sources
	c.sources = append(c.sources, fileSource, memSource)
	dispatcher.AddHandlerFunc(func(r *dispatch.Request) error {
		if r.Type == "SHUTDOWN" {
			log.Printf("coordinator status: waiting for services to shutdown")
			wg := sync.WaitGroup{}
			for _, src := range c.sources {
				wg.Add(1)
				go func() {
					src.Wait()
					wg.Done()
				}()
			}
			wg.Wait()
			close(c.done)
		}
		return nil
	})

	// httpmux handlers
	apiServer.Serve("/api", c.port)
	websocketServer.Serve("/connect", c.port)
	filesServer.Serve("/")

	// special helper endpoint
	http.HandleFunc("/getid", func(w http.ResponseWriter, r *http.Request) {
		p := r.FormValue("path")
		if p == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id := c.GetID(p)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		io.WriteString(w, id)
	})

	http.Serve(c.listener, nil)
}


// GetID returns the id of a file
func (c *Coordinator) GetID(path string) string {
	log.Printf("coordinator status: looking for unique ID: path=%q\n", path)
	for _, source := range c.sources {
		id, err := source.GetID(path)
		if err != nil {
			log.Printf("coordinator warning: %v\n", err)
		}
		if err == nil {
			return id
		}
	}
	log.Printf("coordinator error: could not find unique ID: path=%q\n", path)
	return ""
}

// Wait blocks until server shutdown
func (c *Coordinator) Wait() {
	<-c.done
	log.Println("coordinator status: shutdown complete")
}
