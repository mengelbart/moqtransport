package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeUpdateMessageAppend(t *testing.T) {
	cases := []struct {
		sum    SubscribeUpdateMessage
		buf    []byte
		expect []byte
	}{
		{
			sum: SubscribeUpdateMessage{
				RequestID: 0,
				StartLocation: Location{
					Group:  0,
					Object: 0,
				},
				EndGroup:           0,
				SubscriberPriority: 0,
				Forward:            0,
				Parameters:         KVPList{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			sum: SubscribeUpdateMessage{
				RequestID: 1,
				StartLocation: Location{
					Group:  2,
					Object: 3,
				},
				EndGroup:           4,
				SubscriberPriority: 5,
				Forward:            1,
				Parameters:         KVPList{KeyValuePair{Type: PathParameterKey, ValueBytes: []byte("A")}},
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x01, 0x01, 0x01, 0x01, 'A'},
		},
		{
			sum: SubscribeUpdateMessage{
				RequestID: 1,
				StartLocation: Location{
					Group:  2,
					Object: 3,
				},
				EndGroup:           4,
				SubscriberPriority: 5,
				Forward:            1,
				Parameters:         KVPList{KeyValuePair{Type: PathParameterKey, ValueBytes: []byte("A")}},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x01, 0x02, 0x03, 0x04, 0x05, 0x01, 0x01, 0x01, 0x01, 'A'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sum.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeUpdateMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *SubscribeUpdateMessage
		err    error
	}{
		{
			data:   nil,
			expect: &SubscribeUpdateMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &SubscribeUpdateMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x00, 0x01, 0x02},
			expect: &SubscribeUpdateMessage{
				RequestID: 0,
				StartLocation: Location{
					Group:  1,
					Object: 2,
				},
				EndGroup:           0,
				SubscriberPriority: 0,
				Forward:            0,
				Parameters:         nil,
			},
			err: io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x01, 0x01, 0x01, 0x01, 'P'},
			expect: &SubscribeUpdateMessage{
				RequestID: 1,
				StartLocation: Location{
					Group:  2,
					Object: 3,
				},
				EndGroup:           4,
				SubscriberPriority: 5,
				Forward:            1,
				Parameters:         KVPList{KeyValuePair{Type: PathParameterKey, ValueBytes: []byte("P")}},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &SubscribeUpdateMessage{}
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
