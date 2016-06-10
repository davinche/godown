package sources

// Source is the interface for a markdown file provider
type Source interface {
	GetID(string) (string, error)
}

// RenderFormat is the struct that holds the rendered markdown
type RenderFormat struct {
	Render string `json:"render"`
}
