package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTriePut(t *testing.T) {
	n := NewTrie[string, string]()
	assert.Equal(t, &Trie[string, string]{
		children: map[string]*Trie[string, string]{},
		value:    "",
		terminal: false,
	}, n)

	n.Put([]string{"first"}, "s1")
	assert.Equal(t, &Trie[string, string]{
		children: map[string]*Trie[string, string]{
			"first": {
				children: map[string]*Trie[string, string]{},
				value:    "s1",
				terminal: true,
			},
		},
		value: "",
	}, n)

	n.Put([]string{"second"}, "s2")
	assert.Equal(t, &Trie[string, string]{
		children: map[string]*Trie[string, string]{
			"first": {
				children: map[string]*Trie[string, string]{},
				value:    "s1",
				terminal: true,
			},
			"second": {
				children: map[string]*Trie[string, string]{},
				value:    "s2",
				terminal: true,
			},
		},
		value: "",
	}, n)

	n.Put([]string{"first", "key"}, "nested")
	assert.Equal(t, &Trie[string, string]{
		children: map[string]*Trie[string, string]{
			"first": {
				children: map[string]*Trie[string, string]{
					"key": {
						children: map[string]*Trie[string, string]{},
						value:    "nested",
						terminal: true,
					},
				},
				value:    "s1",
				terminal: true,
			},
			"second": {
				children: map[string]*Trie[string, string]{},
				value:    "s2",
				terminal: true,
			},
		},
		value: "",
	}, n)
}

func TestTrieGet(t *testing.T) {
	n := &Trie[string, string]{
		children: map[string]*Trie[string, string]{
			"first": {
				children: map[string]*Trie[string, string]{
					"key": {
						children: map[string]*Trie[string, string]{},
						value:    "nested",
						terminal: true,
					},
				},
				value:    "s1",
				terminal: true,
			},
			"second": {
				children: map[string]*Trie[string, string]{},
				value:    "s2",
				terminal: true,
			},
			"third": {
				children: map[string]*Trie[string, string]{
					"key": {
						children: map[string]*Trie[string, string]{},
						value:    "nested3",
						terminal: true,
					},
				},
				value:    "",
				terminal: false,
			},
		},
		value: "",
	}

	v, ok := n.Get([]string{"first"})
	assert.True(t, ok)
	assert.Equal(t, "s1", v)

	v, ok = n.Get([]string{"first", "key"})
	assert.True(t, ok)
	assert.Equal(t, "nested", v)

	v, ok = n.Get([]string{"second"})
	assert.True(t, ok)
	assert.Equal(t, "s2", v)

	v, ok = n.Get([]string{"third"})
	assert.False(t, ok)
	assert.Equal(t, "", v)

	v, ok = n.Get([]string{"third", "key"})
	assert.True(t, ok)
	assert.Equal(t, "nested3", v)
}

func TestTrieRemove(t *testing.T) {
	n := &Trie[string, string]{
		children: map[string]*Trie[string, string]{
			"first": {
				children: map[string]*Trie[string, string]{
					"key": {
						children: map[string]*Trie[string, string]{},
						value:    "nested",
						terminal: true,
					},
				},
				value:    "s1",
				terminal: true,
			},
			"second": {
				children: map[string]*Trie[string, string]{},
				value:    "s2",
				terminal: true,
			},
			"third": {
				children: map[string]*Trie[string, string]{
					"key": {
						children: map[string]*Trie[string, string]{},
						value:    "nested3",
						terminal: true,
					},
				},
				value:    "",
				terminal: false,
			},
		},
		value: "",
	}

	assert.False(t, n.Remove(nil))
}
