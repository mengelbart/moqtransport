package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrie(t *testing.T) {
	t.Run("insert", func(t *testing.T) {
		trie := NewTrie[any]()
		assert.Equal(t, &Trie[any]{
			children: map[string]Trie[any]{},
		}, trie)
		trie.Insert([]string{"namespace.org", "meeting123"}, nil)
		assert.Equal(t, &Trie[any]{
			children: map[string]Trie[any]{
				"namespace.org": {
					children: map[string]Trie[any]{
						"meeting123": {
							children: map[string]Trie[any]{},
						},
					},
				},
			},
		}, trie)

		trie.Insert([]string{"namespace.org", "meetingABC"}, nil)
		assert.Equal(t, &Trie[any]{
			children: map[string]Trie[any]{
				"namespace.org": {
					children: map[string]Trie[any]{
						"meeting123": {
							children: map[string]Trie[any]{},
						},
						"meetingABC": {
							children: map[string]Trie[any]{},
						},
					},
				},
			},
		}, trie)

		trie.Insert([]string{"namespace.com", "livestream", "video"}, nil)
		assert.Equal(t, &Trie[any]{
			children: map[string]Trie[any]{
				"namespace.org": {
					children: map[string]Trie[any]{
						"meeting123": {
							children: map[string]Trie[any]{},
						},
						"meetingABC": {
							children: map[string]Trie[any]{},
						},
					},
				},
				"namespace.com": {
					children: map[string]Trie[any]{
						"livestream": {
							children: map[string]Trie[any]{
								"video": {
									children: map[string]Trie[any]{},
									value:    nil,
								},
							},
						},
					},
				},
			},
		}, trie)
	})
}
