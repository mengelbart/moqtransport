package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyValuePairAppend(t *testing.T) {
	cases := []struct {
		p      KeyValuePair
		buf    []byte
		expect []byte
	}{
		{
			p: KeyValuePair{
				Type:  1,
				Bytes: []byte(""),
			},
			buf:    nil,
			expect: []byte{0x01, 0x00},
		},
		{
			p: KeyValuePair{
				Type:  1,
				Bytes: []byte("A"),
			},
			buf:    nil,
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p: KeyValuePair{
				Type:  1,
				Bytes: []byte("A"),
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x01, 0x01, 'A'},
		},
		{
			p: KeyValuePair{
				Type:   2,
				Varint: uint64(1),
			},
			buf:    nil,
			expect: []byte{0x02, 0x01},
		},
		{
			p: KeyValuePair{
				Type:   MaxRequestIDParameterKey,
				Varint: uint64(2),
			},
			buf:    []byte{},
			expect: []byte{0x02, 0x02},
		},
		{
			p: KeyValuePair{
				Type:   MaxRequestIDParameterKey,
				Varint: uint64(3),
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x02, 0x03},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.p.append_v18(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseKeyValuePair(t *testing.T) {
	cases := []struct {
		data     []byte
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
			data: []byte{0x01, 0x08, 'A'},
			expect: KeyValuePair{
				Type: PathParameterKey,
			},
			err:      io.ErrUnexpectedEOF,
			consumed: 2,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			r := newTestBoundedReader(t, tc.data, int64(len(tc.data)))

			res := KeyValuePair{}
			err := res.parse_v18(r)
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
