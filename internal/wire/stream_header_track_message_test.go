package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamHeaderTrackMessageAppend(t *testing.T) {
	cases := []struct {
		shtm   StreamHeaderTrackMessage
		buf    []byte
		expect []byte
	}{
		{
			shtm: StreamHeaderTrackMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				ObjectSendOrder: 0,
			},
			buf:    []byte{},
			expect: []byte{0x40, 0x50, 0x00, 0x00, 0x00},
		},
		{
			shtm: StreamHeaderTrackMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				ObjectSendOrder: 3,
			},
			buf:    []byte{},
			expect: []byte{0x40, 0x50, 0x01, 0x02, 0x03},
		},
		{
			shtm: StreamHeaderTrackMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				ObjectSendOrder: 3,
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x40, 0x50, 0x01, 0x02, 0x03},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.shtm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseStreamHeaderTrackMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *StreamHeaderTrackMessage
		err    error
	}{
		{
			data:   nil,
			expect: &StreamHeaderTrackMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &StreamHeaderTrackMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 0x03},
			expect: &StreamHeaderTrackMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				ObjectSendOrder: 3,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &StreamHeaderTrackMessage{}
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
