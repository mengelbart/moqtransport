package wire

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendVarIntString(t *testing.T) {
	cases := []struct {
		buf    []byte
		in     string
		expect []byte
	}{
		{
			buf:    nil,
			in:     "",
			expect: []byte{0x00},
		},
		{
			buf:    []byte{0x01, 0x02, 0x03},
			in:     "",
			expect: []byte{0x01, 0x02, 0x03, 0x00},
		},
		{
			buf:    []byte{},
			in:     "hello world",
			expect: append([]byte{0x0b}, []byte("hello world")...),
		},
		{
			buf:    []byte{0x01, 0x02, 0x03},
			in:     "hello world",
			expect: append([]byte{0x01, 0x02, 0x03, 0x0b}, []byte("hello world")...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := appendVarIntBytes(tc.buf, []byte(tc.in))
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestVarIntStringLen(t *testing.T) {
	cases := []struct {
		in     string
		expect uint64
	}{
		{
			in:     "",
			expect: 1,
		},
		{
			in:     "hello world",
			expect: 1 + 11,
		},
		{
			in:     strings.Repeat("AAAAAAAA", 20),
			expect: 2 + 160,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := varIntBytesLen(tc.in)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseVarIntBytes(t *testing.T) {
	cases := []struct {
		data   []byte
		expect []byte
		err    error
		n      int
	}{
		{
			data:   nil,
			expect: []byte(""),
			err:    io.EOF,
			n:      0,
		},
		{
			data:   []byte{},
			expect: []byte(""),
			err:    io.EOF,
			n:      0,
		},
		{
			data:   append([]byte{0x01}, "A"...),
			expect: []byte("A"),
			err:    nil,
			n:      2,
		},
		{
			data:   append([]byte{0x04}, "ABC"...),
			expect: []byte(""),
			err:    io.ErrUnexpectedEOF,
			n:      4,
		},
		{
			data:   append([]byte{0x02}, "ABC"...),
			expect: []byte("AB"),
			err:    nil,
			n:      3,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, n, err := parseVarIntBytes(tc.data)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, tc.n, n)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

}
