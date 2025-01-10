package container

type Trie[K comparable, V any] struct {
	children map[K]*Trie[K, V]
	value    V
	terminal bool
}

func NewTrie[K comparable, V any]() *Trie[K, V] {
	return &Trie[K, V]{
		children: map[K]*Trie[K, V]{},
		value:    *new(V),
		terminal: false,
	}
}

func (t *Trie[K, V]) Put(key []K, value V) {
	node := t
	for _, prefix := range key {
		next, ok := node.children[prefix]
		if !ok {
			next = NewTrie[K, V]()
			node.children[prefix] = next
		}
		node = next
	}
	node.value = value
	node.terminal = true
}

func (t *Trie[K, V]) Get(key []K) (V, bool) {
	if key == nil {
		return *new(V), false
	}
	node := t
	for _, prefix := range key {
		next, ok := node.children[prefix]
		if !ok {
			return *new(V), false
		}
		node = next
	}
	if !node.terminal {
		return *new(V), false
	}
	return node.value, true
}

func (t *Trie[K, V]) Remove(key []K) bool {
	return false
}
