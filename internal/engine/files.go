package engine

import "strings"

// Files resolves the disc namespaces supplied by Godot, which may share digital
// LOCAL paths or refer to distinct original CD files. Cached bytes belong
// to the provider; asset edits must use df.File.Patch's copy-on-write path.
type Files struct {
	Index map[string]string
	Read  func(string) ([]byte, error)
	disc  int
	cache map[string][]byte
}

func NewFiles(index map[string]string, read func(string) ([]byte, error)) *Files {
	return &Files{Index: index, Read: read, disc: 1, cache: map[string][]byte{}}
}
func (f *Files) ActiveDisc() int { return f.disc }
func (f *Files) SetDisc(disc int) {
	if disc == 1 || disc == 2 {
		f.disc = disc
	}
}
func (f *Files) key(name string) string {
	name = strings.ToLower(name[strings.LastIndexAny(name, "\\/:")+1:])
	if f.disc == 2 {
		return "2/" + name
	}
	return "1/" + name
}
func (f *Files) Has(name string) bool { _, ok := f.cache[f.key(name)]; return ok }
func (f *Files) Evict(name string) int {
	key := f.key(name)
	n := len(f.cache[key])
	delete(f.cache, key)
	return n
}
func (f *Files) Provide(name string) ([]byte, error) {
	key := f.key(name)
	if data, ok := f.cache[key]; ok {
		return data, nil
	}
	path := f.Index[key]
	if path == "" {
		other := "2/"
		if f.disc == 2 {
			other = "1/"
		}
		path = f.Index[other+key[2:]]
	}
	if path == "" {
		return nil, nil
	}
	data, err := f.Read(path)
	if err == nil && data != nil {
		f.cache[key] = data
	}
	return data, err
}
func (f *Files) Clear() { clear(f.cache) }
