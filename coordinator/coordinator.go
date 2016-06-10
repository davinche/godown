package coordinator

import (
	"net"
	"net/http"
	"strconv"

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
	dispatcher.AddHandler(fileSource)

	// Track sources
	c.sources = append(c.sources, fileSource)
	dispatcher.AddHandlerFunc(func(r *dispatch.Request) error {
		// close(c.done)
		return nil
	})

	// httpmux handlers
	apiServer.Serve("/", c.port)
	websocketServer.Serve("/connect", c.port)
	filesServer.Serve("/static/")
	http.Serve(c.listener, nil)
}

// GetID returns the id of a file
func (c *Coordinator) GetID(path string) string {
	for _, source := range c.sources {
		id, err := source.GetID(path)
		if err == nil {
			return id
		}
	}
	return ""
}

// Wait blocks until server shutdown
func (c *Coordinator) Wait() {
	<-c.done
}
