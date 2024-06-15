package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnounceOkMessageAppend(t *testing.T) {
	cases := []struct {
		aom    AnnounceOkMessage
		buf    []byte
		expect []byte
	}{
		{
			aom: AnnounceOkMessage{
				TrackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(announceOkMessageType), 0x00,
			},
		},
		{
			aom: AnnounceOkMessage{
				TrackNamespace: "tracknamespace",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(announceOkMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aom.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceOkMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *AnnounceOkMessage
		err    error
	}{
		{
			data:   nil,
			expect: &AnnounceOkMessage{},
			err:    io.EOF,
		},
		{
			data: append([]byte{0x0E}, "tracknamespace"...),
			expect: &AnnounceOkMessage{
				TrackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			data: append([]byte{0x05}, "tracknamespace"...),
			expect: &AnnounceOkMessage{
				TrackNamespace: "track",
			},
			err: nil,
		},
		{
			data:   append([]byte{0x0F}, "tracknamespace"...),
			expect: &AnnounceOkMessage{},
			err:    io.ErrUnexpectedEOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &AnnounceOkMessage{}
			err := res.parse(reader)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
