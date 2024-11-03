package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoAwayMessageAppend(t *testing.T) {
	cases := []struct {
		gam    GoAwayMessage
		buf    []byte
		expect []byte
	}{
		{
			gam: GoAwayMessage{
				NewSessionURI: "",
			},
			buf: []byte{},
			expect: []byte{
				0x00,
			},
		},
		{
			gam: GoAwayMessage{
				NewSessionURI: "uri",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, 0x03, 'u', 'r', 'i',
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.gam.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseGoAwayMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *GoAwayMessage
		err    error
	}{
		{
			data:   nil,
			expect: &GoAwayMessage{},
			err:    io.EOF,
		},
		{
			data: append([]byte{0x03}, "uri"...),
			expect: &GoAwayMessage{
				NewSessionURI: "uri",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &GoAwayMessage{}
			err := res.parse(tc.data)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
