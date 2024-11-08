package container

type Trie[V any] struct {
	children map[string]Trie[V]
	value    V
}

func NewTrie[V any]() *Trie[V] {
	return &Trie[V]{
		children: map[string]Trie[V]{},
	}
}

func (n *Trie[V]) Insert(key []string, value V) {
	node := n
	for _, prefix := range key {
		next, ok := n.children[prefix]
		if !ok {
			next = Trie[V]{
				children: map[string]Trie[V]{},
			}
			node.children[prefix] = next
		}
		node = &next
	}
}

func (n *Trie[V]) Get(v []string) (V, bool) {
	return *new(V), false
}

func (n *Trie[V]) Delete(v []string) {

}
