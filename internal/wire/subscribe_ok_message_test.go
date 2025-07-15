package wire

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeOkMessageAppend(t *testing.T) {
	cases := []struct {
		som    SubscribeOkMessage
		buf    []byte
		expect []byte
	}{
		{
			som: SubscribeOkMessage{
				RequestID:     0,
				TrackAlias:    0,
				Expires:       0,
				GroupOrder:    1,
				ContentExists: true,
				LargestLocation: Location{
					Group:  1,
					Object: 2,
				},
				Parameters: KVPList{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 0x02, 0x00,
			},
		},
		{
			som: SubscribeOkMessage{
				RequestID:     17,
				TrackAlias:    0,
				Expires:       1000,
				GroupOrder:    1,
				ContentExists: true,
				LargestLocation: Location{
					Group:  1,
					Object: 2,
				},
				Parameters: KVPList{},
			},
			buf:    []byte{},
			expect: []byte{0x11, 0x00, 0x43, 0xe8, 0x01, 0x01, 0x01, 0x02, 0x00},
		},
		{
			som: SubscribeOkMessage{
				RequestID:     17,
				TrackAlias:    0,
				Expires:       1000,
				GroupOrder:    2,
				ContentExists: true,
				LargestLocation: Location{
					Group:  1,
					Object: 2,
				},
				Parameters: KVPList{},
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, 0x11, 0x00, 0x43, 0xe8, 0x02, 0x01, 0x01, 0x02, 0x00},
		},
		{
			som: SubscribeOkMessage{
				RequestID:     0,
				TrackAlias:    0,
				Expires:       0,
				GroupOrder:    2,
				ContentExists: false,
				LargestLocation: Location{
					Group:  1,
					Object: 2,
				},
				Parameters: KVPList{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00, 0x00, 0x02, 0x00, 0x00,
			},
		},
		{
			som: SubscribeOkMessage{
				RequestID:     17,
				TrackAlias:    0,
				Expires:       1000,
				GroupOrder:    1,
				ContentExists: false,
				LargestLocation: Location{
					Group:  1,
					Object: 2,
				},
				Parameters: KVPList{},
			},
			buf:    []byte{},
			expect: []byte{0x11, 0x00, 0x43, 0xe8, 0x01, 0x00, 0x00},
		},
		{
			som: SubscribeOkMessage{
				RequestID:     17,
				TrackAlias:    0,
				Expires:       1000,
				GroupOrder:    2,
				ContentExists: false,
				LargestLocation: Location{
					Group:  1,
					Object: 2,
				},
				Parameters: KVPList{},
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, 0x11, 0x00, 0x43, 0xe8, 0x02, 0x00, 0x00},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.som.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeOkMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *SubscribeOkMessage
		err    error
	}{
		{
			data:   nil,
			expect: &SubscribeOkMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &SubscribeOkMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x00, 0x10, 0x01, 0x00, 0x00},
			expect: &SubscribeOkMessage{
				RequestID:     1,
				TrackAlias:    0,
				Expires:       0x10 * time.Millisecond,
				GroupOrder:    1,
				ContentExists: false,
				LargestLocation: Location{
					Group:  0,
					Object: 0,
				},
				Parameters: KVPList{},
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x00, 0x10, 0x02, 0x01, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				RequestID:     1,
				TrackAlias:    0,
				Expires:       0x10 * time.Millisecond,
				GroupOrder:    2,
				ContentExists: true,
				LargestLocation: Location{
					Group:  1,
					Object: 2,
				},
				Parameters: KVPList{},
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x00, 0x10, 0x02, 0x08, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				RequestID:     1,
				TrackAlias:    0,
				Expires:       0x10 * time.Millisecond,
				GroupOrder:    2,
				ContentExists: false,
				LargestLocation: Location{
					Group:  0,
					Object: 0,
				},
				Parameters: nil,
			},
			err: errInvalidContentExistsByte,
		},
		{
			data: []byte{0x01, 0x00, 0x10, 0x09, 0x08, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				RequestID:  1,
				TrackAlias: 0,
				Expires:    0x10 * time.Millisecond,
				GroupOrder: 9,
			},
			err: errInvalidGroupOrder,
		},
		{
			data: []byte{0x01, 0x00, 0x10, 0x03, 0x08, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				RequestID:  1,
				TrackAlias: 0,
				Expires:    0x10 * time.Millisecond,
				GroupOrder: 3,
			},
			err: errInvalidGroupOrder,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &SubscribeOkMessage{}
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
