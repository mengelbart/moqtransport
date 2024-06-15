package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnounceErrorMessageAppend(t *testing.T) {
	cases := []struct {
		aem    AnnounceErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			aem: AnnounceErrorMessage{
				TrackNamespace: "",
				ErrorCode:      0,
				ReasonPhrase:   "",
			},
			buf: []byte{},
			expect: []byte{
				byte(announceErrorMessageType), 0x00, 0x00, 0x00,
			},
		},
		{
			aem: AnnounceErrorMessage{
				TrackNamespace: "trackname",
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(announceErrorMessageType), 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
		{
			aem: AnnounceErrorMessage{
				TrackNamespace: "trackname",
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, byte(announceErrorMessageType), 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aem.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceErrorMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *AnnounceErrorMessage
		err    error
	}{
		{
			data:   nil,
			expect: &AnnounceErrorMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x02, 'n', 's', 0x03},
			expect: &AnnounceErrorMessage{
				TrackNamespace: "ns",
				ErrorCode:      3,
				ReasonPhrase:   "",
			},
			err: io.EOF,
		},
		{
			data: append(append(append([]byte{0x0e}, "tracknamespace"...), 0x01, 0x0d), "reason phrase"...),
			expect: &AnnounceErrorMessage{
				TrackNamespace: "tracknamespace",
				ErrorCode:      1,
				ReasonPhrase:   "reason phrase",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &AnnounceErrorMessage{}
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
