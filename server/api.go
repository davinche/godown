package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/davinche/godown/dispatch"
	"github.com/kardianos/osext"
)

var templates *template.Template

func init() {
	// parse the html template
	binDir, err := osext.ExecutableFolder()
	if err != nil {
		fmt.Println("error: could not determine binary folder")
		os.Exit(1)
	}
	templates = template.Must(template.ParseFiles(binDir + "/index.html"))
}

// API is the server that processes user commands
type API struct {
	prefix     string
	port       int
	dispatcher *dispatch.Dispatcher
}

// NewAPI is the constructor for a new api server
func NewAPI(d *dispatch.Dispatcher) *API {
	return &API{
		dispatcher: d,
	}
}

// Serve starts handling requests at a given url
func (a *API) Serve(prefix string, port int) {
	a.prefix = prefix
	a.port = port
	http.HandleFunc(prefix, a.serve)
}

func decodeFilePath(r io.Reader) (string, error) {
	s := struct{ Path string }{}
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&s); err != nil {
		return "", err
	}
	return s.Path, nil
}

func (a *API) serve(w http.ResponseWriter, r *http.Request) {
	// Are we adding a new file?
	if r.Method == "POST" {
		defer r.Body.Close()
		filePath, err := decodeFilePath(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		done, errCh := a.dispatcher.Dispatch("FILE_ADD", filePath)
		select {
		case <-done:
			w.WriteHeader(http.StatusOK)
		case err := <-errCh:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusRequestTimeout)
		}
		return
	}

	id := r.FormValue("id")
	if r.Method == "DELETE" {
		// shutdown the server
		if id == "" {
			done, _ := a.dispatcher.Dispatch("SHUTDOWN", "")
			<-done
			w.WriteHeader(http.StatusOK)
			os.Exit(0)
			return
		}

		// delete file from tracking
		done, errCh := a.dispatcher.Dispatch("FILE_DELETE", id)
		select {
		case <-done:
			w.WriteHeader(http.StatusOK)
		case err := <-errCh:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusRequestTimeout)
		}
		return
	}

	// Render in memory data
	if r.Method == "PUT" {
		if id == "" {
			http.Error(w, "unique identifier for the markdown file required", http.StatusBadRequest)
			return
		}

		defer r.Body.Close()
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "could not read body data", http.StatusBadRequest)
			return
		}

		done, errCh := a.dispatcher.Dispatch("MEM_ADD", string(data))
		select {
		case <-done:
			w.WriteHeader(http.StatusOK)
		case err := <-errCh:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusRequestTimeout)
		}
		return
	}

	// Render the HTML Page from the browser if get request
	if id == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// serving the file
	var host string
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = "localhost"
	}
	tStruct := struct {
		Host   string
		Port   int
		FileID string
	}{host, a.port, id}
	templates.ExecuteTemplate(w, "index.html", tStruct)
}
