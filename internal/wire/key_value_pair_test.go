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
				Type:       1,
				ValueBytes: []byte(""),
			},
			buf:    nil,
			expect: []byte{0x01, 0x00},
		},
		{
			p: KeyValuePair{
				Type:       1,
				ValueBytes: []byte("A"),
			},
			buf:    nil,
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p: KeyValuePair{
				Type:       1,
				ValueBytes: []byte("A"),
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x01, 0x01, 'A'},
		},
		{
			p: KeyValuePair{
				Type:        2,
				ValueVarInt: uint64(1),
			},
			buf:    nil,
			expect: []byte{0x02, 0x01},
		},
		{
			p: KeyValuePair{
				Type:        MaxRequestIDParameterKey,
				ValueVarInt: uint64(2),
			},
			buf:    []byte{},
			expect: []byte{0x02, 0x02},
		},
		{
			p: KeyValuePair{
				Type:        MaxRequestIDParameterKey,
				ValueVarInt: uint64(3),
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x02, 0x03},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.p.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseKeyValuePair(t *testing.T) {
	cases := []struct {
		data   []byte
		expect KeyValuePair
		err    error
		n      int
	}{
		{
			data: []byte{byte(MaxRequestIDParameterKey), 0x01},
			expect: KeyValuePair{
				Type:        MaxRequestIDParameterKey,
				ValueVarInt: uint64(1),
			},
			err: nil,
			n:   2,
		},
		{
			data: append([]byte{byte(PathParameterKey), 11}, "/path/param"...),
			expect: KeyValuePair{
				Type:       1,
				ValueBytes: []byte("/path/param"),
			},
			err: nil,
			n:   13,
		},
		{
			data:   []byte{},
			expect: KeyValuePair{},
			err:    io.EOF,
			n:      0,
		},
		{
			data: []byte{0x05, 0x01, 0x00},
			expect: KeyValuePair{
				Type:       5,
				ValueBytes: []byte{0x00},
			},
			err: nil,
			n:   3,
		},
		{
			data: []byte{0x01, 0x01, 'A'},
			expect: KeyValuePair{
				Type:       PathParameterKey,
				ValueBytes: []byte("A"),
			},
			err: nil,
			n:   3,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := KeyValuePair{}
			n, err := res.parse(tc.data)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, tc.n, n)
			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
