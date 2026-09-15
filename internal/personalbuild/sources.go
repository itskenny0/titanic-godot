package personalbuild

import (
	"fmt"
	"io"
	"os"
)

// GameSource is a read-only range in a loose file or disc image.
type GameSource struct {
	Name, Path   string
	Offset, Size int64
}

func GameSources(root, manifest string) ([]GameSource, error) {
	items, err := inventory(root, manifest)
	if err != nil {
		return nil, err
	}
	result := make([]GameSource, 0, len(items))
	for _, a := range items {
		result = append(result, GameSource{a.name, a.source, a.offset, a.size})
	}
	return result, nil
}
func (s GameSource) Read() ([]byte, error) {
	if s.Size < 0 || s.Size > 512<<20 || s.Offset < 0 {
		return nil, fmt.Errorf("invalid or oversized game resource: %s", s.Name)
	}
	f, err := os.Open(s.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.NewSectionReader(f, s.Offset, s.Size))
}
