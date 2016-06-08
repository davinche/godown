package markdown

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
)

// File holds the path to a file it's identifer. The identifier is used
// to communicate to the web server which file to serve.
type File struct {
	id   string
	Path string
}

// GetID returns the identifier for the file
func (f *File) GetID() string {
	return f.id
}

// NewFile is the constructor for the object that holds the filepath
// and it's identifier
func NewFile(path string) (*File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("%x", sha1.Sum([]byte(absPath)))
	return &File{
		Path: absPath,
		id:   id,
	}, nil
}
