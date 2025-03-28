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
				SubscribeID:        0,
				StartGroup:         0,
				StartObject:        0,
				EndGroup:           0,
				SubscriberPriority: 0,
				Parameters:         map[uint64]Parameter{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			sum: SubscribeUpdateMessage{
				SubscribeID:        1,
				StartGroup:         2,
				StartObject:        3,
				EndGroup:           4,
				SubscriberPriority: 5,
				Parameters:         Parameters{PathParameterKey: StringParameter{Type: PathParameterKey, Value: "A"}},
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x01, 0x01, 0x01, 'A'},
		},
		{
			sum: SubscribeUpdateMessage{
				SubscribeID:        1,
				StartGroup:         2,
				StartObject:        3,
				EndGroup:           4,
				SubscriberPriority: 5,
				Parameters:         Parameters{PathParameterKey: StringParameter{Type: PathParameterKey, Value: "A"}},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x01, 0x02, 0x03, 0x04, 0x05, 0x01, 0x01, 0x01, 'A'},
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
				SubscribeID:        0,
				StartGroup:         1,
				StartObject:        2,
				EndGroup:           0,
				SubscriberPriority: 0,
				Parameters:         nil,
			},
			err: io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x01, 0x02, 0x01, 'P'},
			expect: &SubscribeUpdateMessage{
				SubscribeID:        1,
				StartGroup:         2,
				StartObject:        3,
				EndGroup:           4,
				SubscriberPriority: 5,
				Parameters: Parameters{
					AuthorizationParameterKey: StringParameter{
						Type:  AuthorizationParameterKey,
						Name:  "authorization_info",
						Value: "P",
					},
				},
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
