package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrackStatusMessageAppend(t *testing.T) {
	cases := []struct {
		tsm    TrackStatusMessage
		buf    []byte
		expect []byte
	}{
		{
			tsm: TrackStatusMessage{
				TrackNamespace: []string{""},
				TrackName:      "",
				StatusCode:     0,
				LastGroupID:    0,
				LastObjectID:   0,
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			tsm: TrackStatusMessage{
				TrackNamespace: []string{"tracknamespace"},
				TrackName:      "track",
				StatusCode:     1,
				LastGroupID:    2,
				LastObjectID:   3,
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x01, 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e', 0x05, 't', 'r', 'a', 'c', 'k', 0x01, 0x02, 0x03},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.tsm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseTrackStatusMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *TrackStatusMessage
		err    error
	}{
		{
			data:   nil,
			expect: &TrackStatusMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &TrackStatusMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x09, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 0x05, 't', 'r', 'a', 'c', 'k', 0x01, 0x02, 0x03},
			expect: &TrackStatusMessage{
				TrackNamespace: []string{"trackname"},
				TrackName:      "track",
				StatusCode:     1,
				LastGroupID:    2,
				LastObjectID:   3,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &TrackStatusMessage{}
			err := res.parse(CurrentVersion, tc.data)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
