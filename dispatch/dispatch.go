package dispatch

import "sync"

// Request represents the incoming action a user wants to perform
type Request struct {
	Type  string
	Value interface{}
}

// A Handler responds to incoming dispatch requests
type Handler interface {
	ServeRequest(r *Request) error
	Wait()
}

// HandlerFunc is an adapter to allow for functions to be used to receive dispatched requests
type HandlerFunc func(r *Request) error

// ServeRequest calls handlerfunc
func (f HandlerFunc) ServeRequest(r *Request) error {
	return f(r)
}

// Wait is noop for handlerfunc
func (f HandlerFunc) Wait() {}

// NewDispatcher is the constructor for a new dispatcher
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		listeners: make([]Handler, 0),
	}

	return d
}

// A Dispatcher is responsible for dispatching requests to handlers
type Dispatcher struct {
	listeners []Handler
	sync.Mutex
}

// AddHandler adds a new handler to the list of request receivers
func (d *Dispatcher) AddHandler(h Handler) {
	d.listeners = append(d.listeners, h)
}

// AddHandlerFunc adds a function to the list of request listeners
func (d *Dispatcher) AddHandlerFunc(f HandlerFunc) {
	adapter := HandlerFunc(f)
	d.AddHandler(adapter)
}

// Dispatch creates a new Request and dispatches it to all handlers
func (d *Dispatcher) Dispatch(rType string, rValue interface{}) (chan struct{}, chan error) {
	var wg sync.WaitGroup
	doneCh := make(chan struct{})
	r := &Request{Type: rType, Value: rValue}
	d.Lock()
	errorCh := make(chan error, len(d.listeners))
	for _, h := range d.listeners {
		wg.Add(1)
		go func(h Handler) {
			if err := h.ServeRequest(r); err != nil {
				errorCh <- err
			}
			wg.Done()
		}(h)
	}
	d.Unlock()
	go func() {
		wg.Wait()
		close(doneCh)
	}()
	return doneCh, errorCh
}
