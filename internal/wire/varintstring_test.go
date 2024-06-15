package wire

import (
	"bufio"
	"bytes"
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
			res := appendVarIntString(tc.buf, tc.in)
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
			res := varIntStringLen(tc.in)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseVarIntString(t *testing.T) {
	cases := []struct {
		data   []byte
		expect string
		err    error
	}{
		{
			data:   nil,
			expect: "",
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: "",
			err:    io.EOF,
		},
		{
			data:   append([]byte{0x01}, "A"...),
			expect: "A",
			err:    nil,
		},
		{
			data:   append([]byte{0x04}, "ABC"...),
			expect: "",
			err:    io.ErrUnexpectedEOF,
		},
		{
			data:   append([]byte{0x02}, "ABC"...),
			expect: "AB",
			err:    nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res, err := parseVarIntString(reader)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

}
