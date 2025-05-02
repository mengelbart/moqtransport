package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamHeaderSubgroupMessageAppend(t *testing.T) {
	cases := []struct {
		shgm   SubgroupHeaderMessage
		buf    []byte
		expect []byte
	}{
		{
			shgm: SubgroupHeaderMessage{
				TrackAlias:        0,
				GroupID:           0,
				SubgroupID:        0,
				PublisherPriority: 0,
			},
			buf:    []byte{},
			expect: []byte{byte(StreamTypeSubgroupSIDExt), 0x00, 0x00, 0x00, 0x00},
		},
		{
			shgm: SubgroupHeaderMessage{
				TrackAlias:        1,
				GroupID:           2,
				SubgroupID:        3,
				PublisherPriority: 4,
			},
			buf:    []byte{},
			expect: []byte{byte(StreamTypeSubgroupSIDExt), 0x01, 0x02, 0x03, 0x04},
		},
		{
			shgm: SubgroupHeaderMessage{
				TrackAlias:        1,
				GroupID:           2,
				SubgroupID:        3,
				PublisherPriority: 4,
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(StreamTypeSubgroupSIDExt), 0x01, 0x02, 0x03, 0x04},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.shgm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseStreamHeaderSubgroupMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *SubgroupHeaderMessage
		err    error
	}{
		{
			data:   nil,
			expect: &SubgroupHeaderMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &SubgroupHeaderMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 0x03, 0x04},
			expect: &SubgroupHeaderMessage{
				TrackAlias:        1,
				GroupID:           2,
				SubgroupID:        3,
				PublisherPriority: 4,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &SubgroupHeaderMessage{}
			err := res.parse(reader, true)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
