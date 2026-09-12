package engine

import "iter"

// orderedMap preserves the insertion order used by scripts and by draw ties.
// Replacing a value leaves its position unchanged; deleting and adding moves it.
type orderedMap[T any] struct {
	data map[string]T
	keys []string
}

func (m *orderedMap[T]) Get(key string) T    { return m.data[key] }
func (m *orderedMap[T]) Has(key string) bool { _, ok := m.data[key]; return ok }
func (m *orderedMap[T]) Set(key string, value T) {
	if m.data == nil {
		m.data = make(map[string]T)
	}
	if !m.Has(key) {
		m.keys = append(m.keys, key)
	}
	m.data[key] = value
}
func (m *orderedMap[T]) Delete(key string) {
	if !m.Has(key) {
		return
	}
	delete(m.data, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}
func (m *orderedMap[T]) Len() int { return len(m.keys) }
func (m *orderedMap[T]) All() iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		for _, key := range m.keys {
			if !yield(key, m.data[key]) {
				return
			}
		}
	}
}
