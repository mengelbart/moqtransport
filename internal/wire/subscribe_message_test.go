package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeMessageAppend(t *testing.T) {
	cases := []struct {
		sm     SubscribeMessage
		buf    []byte
		expect []byte
	}{
		{
			sm: SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{""},
				TrackName:          []byte(""),
				SubscriberPriority: 0,
				GroupOrder:         0,
				FilterType:         0,
				StartGroup:         0,
				StartObject:        0,
				EndGroup:           0,
				Parameters:         Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
			},
		},
		{
			sm: SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				FilterType:         FilterTypeAbsoluteStart,
				StartGroup:         1,
				StartObject:        1,
				EndGroup:           0,
				Parameters:         Parameters{},
			},
			buf:    []byte{},
			expect: append(append([]byte{0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x03, 0x01, 0x01, 0x00),
		},
		{
			sm: SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				FilterType:         FilterTypeAbsoluteRange,
				StartGroup:         1,
				StartObject:        2,
				EndGroup:           3,
				Parameters:         Parameters{AuthorizationParameterKey: StringParameter{Type: AuthorizationParameterKey, Value: "A"}},
			},
			buf:    []byte{},
			expect: append(append([]byte{0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x01, 0x02, 0x04, 0x01, 0x02, 0x03, 0x01, 0x02, 0x01, 'A'}...),
		},
		{
			sm: SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 2,
				GroupOrder:         2,
				FilterType:         0,
				StartGroup:         0,
				StartObject:        0,
				EndGroup:           0,
				Parameters:         Parameters{AuthorizationParameterKey: StringParameter{Type: AuthorizationParameterKey, Value: "A"}},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, 0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x02, 0x02, 0x01, 0x01, 0x02, 0x01, 'A'}...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *SubscribeMessage
		err    error
	}{
		{
			data:   nil,
			expect: &SubscribeMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &SubscribeMessage{},
			err:    io.EOF,
		},
		{
			data: append([]byte{0x09, 0x00, 0x01, byte(len("trackname"))}, "trackname"...),
			expect: &SubscribeMessage{
				SubscribeID:    9,
				TrackAlias:     0,
				TrackNamespace: []string{"trackname"},
				TrackName:      []byte{},
			},
			err: io.EOF,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x00, 0x00, 0x00),
			expect: &SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 0,
				GroupOrder:         0,
			},
			err: errInvalidFilterType,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x01, 0x00),
			expect: &SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				FilterType:         FilterTypeLatestGroup,
				StartGroup:         0,
				StartObject:        0,
				EndGroup:           0,
				Parameters:         Parameters{},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x02, 0x02, 0x03, 0x01, 0x01),
			expect: &SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 2,
				GroupOrder:         2,
				FilterType:         3,
				StartGroup:         1,
				StartObject:        1,
				EndGroup:           0,
				Parameters:         map[uint64]Parameter{},
			},
			err: io.EOF,
		},
		{
			data: append(append([]byte{0x011, 0x012, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x02, 0x02, 0x01, 0x01, 0x02, 0x01, 'A'),
			expect: &SubscribeMessage{
				SubscribeID:        17,
				TrackAlias:         18,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 2,
				GroupOrder:         2,
				FilterType:         1,
				StartGroup:         0,
				StartObject:        0,
				EndGroup:           0,
				Parameters: Parameters{AuthorizationParameterKey: StringParameter{
					Type:  AuthorizationParameterKey,
					Name:  "authorization_info",
					Value: "A",
				}},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x04, 0x01, 0x02, 0x03, 0x01, 0x02, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			expect: &SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				FilterType:         FilterTypeAbsoluteRange,
				StartGroup:         1,
				StartObject:        2,
				EndGroup:           3,
				Parameters: Parameters{AuthorizationParameterKey: StringParameter{
					Type:  AuthorizationParameterKey,
					Name:  "authorization_info",
					Value: "A",
				}},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x03, 0x04, 0x01, 0x02, 0x03, 0x04, 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			expect: &SubscribeMessage{
				SubscribeID:        0,
				TrackAlias:         0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         3,
			},
			err: errInvalidGroupOrder,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &SubscribeMessage{}
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
