package moqtransport

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionsLen(t *testing.T) {
	cases := []struct {
		versions Versions
		expected uint64
	}{
		{
			versions: []Version{},
			expected: 0,
		},
		{
			versions: []Version{Version(0)},
			expected: 1,
		},
		{
			versions: []Version{Version(DRAFT_IETF_MOQ_TRANSPORT_00)},
			expected: 1,
		},
		{
			versions: []Version{Version(1024)},
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
