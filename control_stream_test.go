package moqtransport

import (
	"fmt"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
)

func TestCompileMessage(t *testing.T) {
	cases := []struct {
		msg    wire.Message
		expect []byte
	}{
		{
			msg: &wire.UnsubscribeMessage{
				SubscribeID: 3,
			},
			expect: []byte{0x0a, 0x01, 0x03},
		},
		{
			msg: &wire.SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     [][]byte{{}},
				TrackName:          []byte{},
				SubscriberPriority: 0,
				GroupOrder:         0,
				FilterType:         wire.FilterTypeAbsoluteRange,
				StartGroup:         0,
				StartObject:        0,
				EndGroup:           0,
				EndObject:          0,
				Parameters:         map[uint64]wire.Parameter{},
			},
			expect: []byte{
				0x03,
				0x0D, // length
				0x00,
				0x00,
				0x01, 0x00,
				0x00,
				0x00,
				0x00,
				0x04,
				0x00,
				0x00,
				0x00,
				0x00,
				0x00,
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := compileMessage(tc.msg)
			assert.Equal(t, tc.expect, res)
		})
	}
}
