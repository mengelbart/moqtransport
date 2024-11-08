package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnnounceCancelMessageAppend(t *testing.T) {
	cases := []struct {
		aom    AnnounceCancelMessage
		buf    []byte
		expect []byte
	}{
		{
			aom: AnnounceCancelMessage{
				TrackNamespace: []string{""},
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf: []byte{},
			expect: []byte{
				0x01, 0x00, 0x01, 0x06, 'r', 'e', 'a', 's', 'o', 'n',
			},
		},
		{
			aom: AnnounceCancelMessage{
				TrackNamespace: []string{"tracknamespace"},
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b,
				0x01, 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e',
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aom.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceCancelMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *AnnounceCancelMessage
		err    error
	}{
		{
			data:   nil,
			expect: &AnnounceCancelMessage{},
			err:    io.EOF,
		},
		{
			data: append(
				[]byte{0x01, 0x0E}, append([]byte("tracknamespace"), 0x00, 0x00)...,
			),
			expect: &AnnounceCancelMessage{
				TrackNamespace: []string{"tracknamespace"},
				ErrorCode:      0,
				ReasonPhrase:   "",
			},
			err: nil,
		},
		{
			data: append([]byte{0x01, 0x05}, append([]byte("track"), []byte{0x01, 0x06, 'r', 'e', 'a', 's', 'o', 'n', 'p', 'h', 'r', 'a', 's', 'e'}...)...),
			expect: &AnnounceCancelMessage{
				TrackNamespace: []string{"track"},
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			err: nil,
		},
		{
			data: append([]byte{0x01, 0x0F}, "tracknamespace"...),
			expect: &AnnounceCancelMessage{
				TrackNamespace: []string{},
			},
			err: errLengthMismatch,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &AnnounceCancelMessage{}
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
