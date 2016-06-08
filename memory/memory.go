package memory

import (
	"crypto/sha1"
	"fmt"
)

// File holds the path to a file it's identifer.
type File struct {
	id      string
	Content []byte
}

// GetID returns the identifier for the file
func (f *File) GetID() string {
	return f.id
}

// NewFile is the constructor for the in-memory file
func NewFile(id string, content []byte) (*File, error) {

	hashed := fmt.Sprintf("%x", sha1.Sum([]byte(id)))

	return &File{
		id:      hashed,
		Content: content,
	}, nil
}
