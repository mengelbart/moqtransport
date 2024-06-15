package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnannounceMessageAppend(t *testing.T) {
	cases := []struct {
		uam    UnannounceMessage
		buf    []byte
		expect []byte
	}{
		{
			uam: UnannounceMessage{
				TrackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(unannounceMessageType), 0x00,
			},
		},
		{
			uam: UnannounceMessage{
				TrackNamespace: "tracknamespace",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(unannounceMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.uam.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseUnannounceMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *UnannounceMessage
		err    error
	}{
		{
			data:   nil,
			expect: &UnannounceMessage{},
			err:    io.EOF,
		},
		{
			data: append([]byte{0x0E}, "tracknamespace"...),
			expect: &UnannounceMessage{
				TrackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			data: append([]byte{0x05}, "tracknamespace"...),
			expect: &UnannounceMessage{
				TrackNamespace: "track",
			},
			err: nil,
		},
		{
			data:   append([]byte{0x0F}, "tracknamespace"...),
			expect: &UnannounceMessage{},
			err:    io.ErrUnexpectedEOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &UnannounceMessage{}
			err := res.parse(reader)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				assert.Equal(t, tc.expect, res)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, res)
		})
	}
}
