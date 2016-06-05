package markdown

import (
	"crypto/sha1"
	"fmt"
	"io"
	"path/filepath"
)

// File holds the path to a file it's identifer. The identifier is used
// to communicate to the web server which file to serve.
type File struct {
	Path string
	ID   string
}

// NewFile is the constructor for the object that holds the filepath
// and it's identifier
func NewFile(path string) (*File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	hash := sha1.New()
	if _, err := io.WriteString(hash, absPath); err != nil {
		return nil, err
	}
	ID := fmt.Sprintf("%x", hash.Sum(nil))
	return &File{
		Path: absPath,
		ID:   ID,
	}, nil
}
