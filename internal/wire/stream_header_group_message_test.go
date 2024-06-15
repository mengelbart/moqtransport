package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamHeaderGroupMessageAppend(t *testing.T) {
	cases := []struct {
		shgm   StreamHeaderGroupMessage
		buf    []byte
		expect []byte
	}{
		{
			shgm: StreamHeaderGroupMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectSendOrder: 0,
			},
			buf:    []byte{},
			expect: []byte{0x40, 0x51, 0x00, 0x00, 0x00, 0x00},
		},
		{
			shgm: StreamHeaderGroupMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				GroupID:         3,
				ObjectSendOrder: 4,
			},
			buf:    []byte{},
			expect: []byte{0x40, 0x51, 0x01, 0x02, 0x03, 0x04},
		},
		{
			shgm: StreamHeaderGroupMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				GroupID:         3,
				ObjectSendOrder: 4,
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x40, 0x51, 0x01, 0x02, 0x03, 0x04},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.shgm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseStreamHeaderGroupMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *StreamHeaderGroupMessage
		err    error
	}{
		{
			data:   nil,
			expect: &StreamHeaderGroupMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &StreamHeaderGroupMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 0x03, 0x04},
			expect: &StreamHeaderGroupMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				GroupID:         3,
				ObjectSendOrder: 4,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &StreamHeaderGroupMessage{}
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
