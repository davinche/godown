package server

import (
	"log"
	"net/http"
	"os"

	"github.com/kardianos/osext"
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

// Static is the static files server
type Static struct{}

// NewStatic is the constructor for the static files server
func NewStatic() *Static {
	return &Static{}
}

// Serve registers the static server with the http defaultmux
func (s *Static) Serve(prefix string) {
	static := http.FileServer(http.Dir(binDir + "/static"))
	http.Handle(prefix, http.StripPrefix(prefix, static))
}
