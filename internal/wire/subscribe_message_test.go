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
				RequestID:          0,
				TrackNamespace:     []string{""},
				TrackName:          []byte(""),
				SubscriberPriority: 0,
				GroupOrder:         0,
				Forward:            0,
				FilterType:         0,
				StartLocation: Location{
					Group:  0,
					Object: 0,
				},
				EndGroup:   0,
				Parameters: KVPList{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
			},
		},
		{
			sm: SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				Forward:            0,
				FilterType:         FilterTypeAbsoluteStart,
				StartLocation: Location{
					Group:  1,
					Object: 1,
				},
				EndGroup:   0,
				Parameters: KVPList{},
			},
			buf:    []byte{},
			expect: append(append([]byte{0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x00, 0x03, 0x01, 0x01, 0x00),
		},
		{
			sm: SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				Forward:            0,
				FilterType:         FilterTypeAbsoluteRange,
				StartLocation: Location{
					Group:  1,
					Object: 2,
				},
				EndGroup:   3,
				Parameters: KVPList{KeyValuePair{Type: AuthorizationTokenParameterKey, ValueBytes: []byte("A")}},
			},
			buf:    []byte{},
			expect: append(append([]byte{0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x01, 0x02, 0x00, 0x04, 0x01, 0x02, 0x03, 0x01, 0x03, 0x01, 'A'}...),
		},
		{
			sm: SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 2,
				GroupOrder:         2,
				Forward:            0,
				FilterType:         0,
				StartLocation: Location{
					Group:  0,
					Object: 0,
				},
				EndGroup:   0,
				Parameters: KVPList{KeyValuePair{Type: AuthorizationTokenParameterKey, ValueBytes: []byte("A")}},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, 0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x02, 0x02, 0x00, 0x01, 0x01, 0x03, 0x01, 'A'}...),
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
			data: append([]byte{0x09, 0x01, byte(len("trackname"))}, "trackname"...),
			expect: &SubscribeMessage{
				RequestID:      9,
				TrackNamespace: []string{"trackname"},
				TrackName:      []byte{},
			},
			err: io.EOF,
		},
		{
			data: append(append([]byte{0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x00, 0x00, 0x00),
			expect: &SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 0,
				GroupOrder:         0,
			},
			err: errInvalidFilterType,
		},
		{
			data: append(append([]byte{0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x00, 0x01, 0x00),
			expect: &SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				Forward:            0,
				FilterType:         FilterTypeNextGroupStart,
				EndGroup:           0,
				Parameters:         KVPList{},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x02, 0x02, 0x00, 0x03, 0x01, 0x01),
			expect: &SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 2,
				GroupOrder:         2,
				Forward:            0,
				FilterType:         3,
				StartLocation: Location{
					Group:  1,
					Object: 1,
				},
				EndGroup:   0,
				Parameters: KVPList{},
			},
			err: io.EOF,
		},
		{
			data: append(append([]byte{0x011, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x02, 0x02, 0x00, 0x01, 0x01, 0x03, 0x01, 'A'),
			expect: &SubscribeMessage{
				RequestID:          17,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 2,
				GroupOrder:         2,
				Forward:            0,
				FilterType:         1,
				StartLocation: Location{
					Group:  0,
					Object: 0,
				},
				EndGroup: 0,
				Parameters: KVPList{KeyValuePair{
					Type:       AuthorizationTokenParameterKey,
					ValueBytes: []byte("A"),
				}},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x00, 0x04, 0x01, 0x02, 0x03, 0x01, 0x03, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			expect: &SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         2,
				Forward:            0,
				FilterType:         FilterTypeAbsoluteRange,
				StartLocation: Location{
					Group:  1,
					Object: 2,
				},
				EndGroup: 3,
				Parameters: KVPList{KeyValuePair{
					Type:       AuthorizationTokenParameterKey,
					ValueBytes: []byte("A"),
				}},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x01, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x03, 0x00, 0x04, 0x01, 0x02, 0x03, 0x04, 0x01, 0x03, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			expect: &SubscribeMessage{
				RequestID:          0,
				TrackNamespace:     []string{"ns"},
				TrackName:          []byte("trackname"),
				SubscriberPriority: 1,
				GroupOrder:         3,
				Forward:            0,
			},
			err: errInvalidGroupOrder,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &SubscribeMessage{}
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
