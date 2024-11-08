package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrackStatusRequestMessageAppend(t *testing.T) {
	cases := []struct {
		aom    TrackStatusRequestMessage
		buf    []byte
		expect []byte
	}{
		{
			aom: TrackStatusRequestMessage{
				TrackNamespace: []string{""},
				TrackName:      "",
			},
			buf: []byte{},
			expect: []byte{
				0x01, 0x00, 0x00,
			},
		},
		{
			aom: TrackStatusRequestMessage{
				TrackNamespace: []string{"tracknamespace"},
				TrackName:      "track",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x01, 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e', 0x05, 't', 'r', 'a', 'c', 'k'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aom.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseTrackStatusRequestMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *TrackStatusRequestMessage
		err    error
	}{
		{
			data:   nil,
			expect: &TrackStatusRequestMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e', 0x05, 't', 'r', 'a', 'c', 'k'},
			expect: &TrackStatusRequestMessage{
				TrackNamespace: []string{"tracknamespace"},
				TrackName:      "track",
			},
			err: nil,
		},
		{
			data: append([]byte{0x0f}, "tracknamespace"...),
			expect: &TrackStatusRequestMessage{
				TrackNamespace: []string{},
				TrackName:      "",
			},
			err: errLengthMismatch,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &TrackStatusRequestMessage{}
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
