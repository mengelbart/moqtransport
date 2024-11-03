package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendTuple(t *testing.T) {
	cases := []struct {
		t      Tuple
		buf    []byte
		expect []byte
	}{
		{
			t:      nil,
			buf:    []byte{},
			expect: []byte{0x00},
		},
		{
			t:      [][]byte{{'A'}},
			buf:    []byte{},
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			t:      [][]byte{{'A'}, {'A', 'B', 'C'}},
			buf:    []byte{},
			expect: []byte{0x02, 0x01, 'A', 0x03, 'A', 'B', 'C'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.t.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseTuple(t *testing.T) {
	cases := []struct {
		data   []byte
		expect Tuple
		err    error
		n      int
	}{
		{
			data:   []byte{},
			expect: nil,
			err:    io.EOF,
			n:      0,
		},
		{
			data:   []byte{0x02, 0x01, 'a'},
			expect: [][]byte{{'a'}},
			err:    io.EOF,
			n:      3,
		},
		{
			data:   []byte{0x00},
			expect: [][]byte{},
			err:    nil,
			n:      1,
		},
		{
			data:   []byte{0x01, 0x01, 'a'},
			expect: [][]byte{{'a'}},
			err:    nil,
			n:      3,
		},
		{
			data:   []byte{0x02, 0x01, 'a', 0x02, 'a', 'b'},
			expect: [][]byte{{'a'}, {'a', 'b'}},
			err:    nil,
			n:      6,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, n, err := parseTuple(tc.data)
			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
				assert.Equal(t, tc.expect, res)
				assert.Equal(t, tc.n, n)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, tc.n, n)
		})
	}
}
