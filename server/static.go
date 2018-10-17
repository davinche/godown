package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kardianos/osext"
	"github.com/davinche/godown/dispatch"
)

var binDir string

func init() {
	// parse the html template
	d, err := osext.ExecutableFolder()
	if err != nil {
		log.Println("error: could not determine binary folder")
		os.Exit(1)
	}
	binDir = d
}


// Static is the static files server. It supports serving multiple static folders.
type Static struct{
	pathsToHandle []string
}

// NewStatic is the constructor for the static files server
func NewStatic() *Static {
	return &Static{}
}

// Serve registers the static server with the http defaultmux
func (s *Static) Serve(prefix string) {
	s.AddPath(binDir);
	http.Handle(prefix, s)
}

// ServeHTTP handles HTTP requests to static files
func (s *Static) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	served := false
	for _, path := range s.pathsToHandle {
		requestedFile := path+req.URL.Path
		if (!served) {
			if _, err := os.Stat(requestedFile); !os.IsNotExist(err) {
				log.Println("static file ", req.URL.Path, " served from ", path)
				http.ServeFile(res, req, requestedFile)
				served = true
			}
		}
	}
}

// AddPath Registers a new path to search for static files
func (s *Static) AddPath(pathToHandle string) {
	duplicate := false
	for _, path := range s.pathsToHandle {
		if (path == pathToHandle) {
			duplicate = true
		}
	}

	if (!duplicate) {
		log.Println("Registering ", pathToHandle, " as static file source")
		s.pathsToHandle = append(s.pathsToHandle, pathToHandle)
	}
}

// RemovePath removes a path from static files serving
func (s *Static) RemovePath(pathToRemove string) {
	log.Println("Removing ", pathToRemove, " from static file sources")
	for i, path := range s.pathsToHandle {
		if (path == pathToRemove) {
			lastIndex := len(s.pathsToHandle)-1
			s.pathsToHandle[i] = s.pathsToHandle[lastIndex]
			s.pathsToHandle[lastIndex] = ""
			s.pathsToHandle = s.pathsToHandle[:lastIndex]
		}
	}
}

// ServeRequest handles dispatched messages
func (s *Static) ServeRequest(r *dispatch.Request) error {
	switch r.Type {
	case "FILE_ADD":
		markdownFolder := getOsFolder(r.Value.(string))
		s.AddPath(markdownFolder)
	case "FILE_DELETE":
		markdownFolder := getOsFolder(r.Value.(string))
		s.RemovePath(markdownFolder)
	}
	return nil
}

func  getOsFolder(urlPath string) string {
	return filepath.Dir(filepath.FromSlash(urlPath))
}

// Wait for termination
func (s *Static) Wait() {
}
