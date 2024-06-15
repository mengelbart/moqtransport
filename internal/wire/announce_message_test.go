package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnounceMessageAppend(t *testing.T) {
	cases := []struct {
		am     AnnounceMessage
		buf    []byte
		expect []byte
	}{
		{
			am: AnnounceMessage{
				TrackNamespace: "",
				Parameters:     Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(announceMessageType), 0x00, 0x00,
			},
		},
		{
			am: AnnounceMessage{
				TrackNamespace: "tracknamespace",
				Parameters:     Parameters{},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(announceMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e', 0x00},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.am.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *AnnounceMessage
		err    error
	}{
		{
			data:   nil,
			expect: &AnnounceMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &AnnounceMessage{},
			err:    io.EOF,
		},
		{
			data: append(append([]byte{0x09}, "trackname"...), 0x00),
			expect: &AnnounceMessage{
				TrackNamespace: "trackname",
				Parameters:     Parameters{},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &AnnounceMessage{}
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
