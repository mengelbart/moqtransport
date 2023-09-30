package moqtransport

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionsLen(t *testing.T) {
	cases := []struct {
		versions versions
		expected uint64
	}{
		{
			versions: []version{},
			expected: 0,
		},
		{
			versions: []version{version(0)},
			expected: 1,
		},
		{
			versions: []version{version(DRAFT_IETF_MOQ_TRANSPORT_00)},
			expected: 1,
		},
		{
			versions: []version{version(1024)},
			expected: 2,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.versions.Len()
			assert.Equal(t, tc.expected, res)
		})
	}
}
