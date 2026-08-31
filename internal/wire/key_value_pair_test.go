package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/mengelbart/moqtransport/varint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendKeyValuePairs(t *testing.T) {
	cases := []struct {
		pairs  []KeyValuePair
		buf    []byte
		expect []byte
	}{
		{
			pairs: []KeyValuePair{{
				Type:  1,
				Bytes: []byte(""),
			}},
			buf:    nil,
			expect: []byte{0x01, 0x00},
		},
		{
			pairs: []KeyValuePair{{
				Type:  1,
				Bytes: []byte("A"),
			}},
			buf:    nil,
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			pairs: []KeyValuePair{{
				Type:  1,
				Bytes: []byte("A"),
			}},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x01, 0x01, 'A'},
		},
		{
			pairs: []KeyValuePair{{
				Type:   2,
				Varint: uint64(1),
			}},
			buf:    nil,
			expect: []byte{0x02, 0x01},
		},
		{
			pairs: []KeyValuePair{{
				Type:   MaxRequestIDParameterKey,
				Varint: uint64(2),
			}},
			buf:    []byte{},
			expect: []byte{0x02, 0x02},
		},
		{
			pairs: []KeyValuePair{{
				Type:   MaxRequestIDParameterKey,
				Varint: uint64(3),
			}},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x02, 0x03},
		},
		{
			pairs: []KeyValuePair{
				{Type: 2, Varint: 42},
				{Type: 77, Bytes: []byte("A")},
			},
			buf:    nil,
			expect: []byte{0x02, 0x2a, 0x4b, 0x01, 'A'},
		},
		{
			pairs: []KeyValuePair{
				{Type: 3, Bytes: []byte("A")},
				{Type: 3, Bytes: []byte("B")},
			},
			buf:    nil,
			expect: []byte{0x03, 0x01, 'A', 0x00, 0x01, 'B'},
		},
		{
			pairs: []KeyValuePair{
				{Type: 2, Varint: 42},
				{Type: 1, Bytes: []byte("A")},
			},
			buf:    nil,
			expect: []byte{0x01, 0x01, 'A', 0x01, 0x2a},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			in := append([]KeyValuePair{}, tc.pairs...)
			res := appendKeyValuePairs_v18(tc.buf, tc.pairs)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, in, tc.pairs, "input must not be reordered in place")
		})
	}
}

func TestParseKeyValuePair(t *testing.T) {
	cases := []struct {
		data     []byte
		prev     uint64
		expect   KeyValuePair
		err      error
		consumed int64
	}{
		{
			data: []byte{byte(MaxRequestIDParameterKey), 0x01},
			expect: KeyValuePair{
				Type:   MaxRequestIDParameterKey,
				Varint: uint64(1),
			},
			err:      nil,
			consumed: 2,
		},
		{
			data: append([]byte{byte(PathParameterKey), 11}, "/path/param"...),
			expect: KeyValuePair{
				Type:  1,
				Bytes: []byte("/path/param"),
			},
			err:      nil,
			consumed: 13,
		},
		{
			data:     []byte{},
			expect:   KeyValuePair{},
			err:      io.ErrUnexpectedEOF,
			consumed: 0,
		},
		{
			data: []byte{0x05, 0x01, 0x00},
			expect: KeyValuePair{
				Type:  5,
				Bytes: []byte{0x00},
			},
			err:      nil,
			consumed: 3,
		},
		{
			data: []byte{0x01, 0x01, 'A'},
			expect: KeyValuePair{
				Type:  PathParameterKey,
				Bytes: []byte("A"),
			},
			err:      nil,
			consumed: 3,
		},
		{
			data:     []byte{0x01, 0x08, 'A'},
			expect:   KeyValuePair{},
			err:      io.ErrUnexpectedEOF,
			consumed: 2,
		},
		{
			data: []byte{0x01, 0x2a},
			prev: 1,
			expect: KeyValuePair{
				Type:   2,
				Varint: 42,
			},
			err:      nil,
			consumed: 2,
		},
		{
			data: []byte{0x00, 0x01, 'A'},
			prev: 3,
			expect: KeyValuePair{
				Type:  3,
				Bytes: []byte("A"),
			},
			err:      nil,
			consumed: 3,
		},
		{
			data:     varint.Append(nil, 2),
			prev:     1<<64 - 1,
			expect:   KeyValuePair{},
			err:      errKeyValuePairTypeOverflow,
			consumed: 1,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			r := newTestBoundedReader(t, tc.data, int64(len(tc.data)))

			res, err := parseKeyValuePair_v18(r, tc.prev)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, tc.consumed, int64(len(tc.data))-r.remaining())
			if tc.err != nil {
				assert.ErrorIs(t, err, tc.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseKeyValuePairValueTooLong(t *testing.T) {
	value := make([]byte, maxKeyValuePairValueLength+1)
	data := varint.Append(nil, 1)
	data = varint.Append(data, uint64(len(value)))
	data = append(data, value...)

	r := newTestBoundedReader(t, data, int64(len(data)))
	_, err := parseKeyValuePair_v18(r, 0)
	assert.ErrorIs(t, err, errKeyValuePairValueTooLong)
}

func TestParseKeyValuePairsAccumulatesTypes(t *testing.T) {
	pairs := []KeyValuePair{
		{Type: 2, Varint: 42},
		{Type: 77, Bytes: []byte("A")},
		{Type: 77, Bytes: []byte("B")},
	}
	buf := varint.Append(nil, uint64(len(pairs)))
	buf = appendKeyValuePairs_v18(buf, pairs)

	assert.Equal(t, []byte{
		0x03,       // number of pairs
		0x02, 0x2a, // type 2, varint 42
		0x4b, 0x01, 'A', // delta 75, type 77, one byte
		0x00, 0x01, 'B', // delta 0, type 77, one byte
	}, buf)

	r := newTestBoundedReader(t, buf, int64(len(buf)))
	got, err := parseKeyValuePairsCount_v18(r)
	require.NoError(t, err)
	assert.Equal(t, pairs, got)
}

func TestParseKeyValuePairsRemainingNeedsLength(t *testing.T) {
	_, err := parseKeyValuePairsRemaining_v18(&unboundedReader{})
	assert.ErrorIs(t, err, errNoMessageLength)
}
