package wire

import (
	"bufio"
	"bytes"
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
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "",
				TrackName:      "",
				FilterType:     0,
				StartGroup:     0,
				StartObject:    0,
				EndGroup:       0,
				EndObject:      0,
				Parameters:     Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeMessageType), 0x00, 0x00, 0x00, 0x00, 0x01, 0x00,
			},
		},
		{
			sm: SubscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				FilterType:     FilterTypeAbsoluteStart,
				StartGroup:     1,
				StartObject:    1,
				EndGroup:       0,
				EndObject:      0,
				Parameters:     Parameters{},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeMessageType), 0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x03, 0x01, 0x01, 0x00),
		},
		{
			sm: SubscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				FilterType:     FilterTypeAbsoluteRange,
				StartGroup:     1,
				StartObject:    2,
				EndGroup:       3,
				EndObject:      4,
				Parameters:     Parameters{PathParameterKey: StringParameter{Type: PathParameterKey, Value: "A"}},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeMessageType), 0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x04, 0x01, 0x02, 0x03, 0x04, 0x01, 0x01, 0x01, 'A'}...),
		},
		{
			sm: SubscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				FilterType:     0,
				StartGroup:     0,
				StartObject:    0,
				EndGroup:       0,
				EndObject:      0,
				Parameters:     Parameters{PathParameterKey: StringParameter{Type: PathParameterKey, Value: "A"}},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, byte(subscribeMessageType), 0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x01, 0x01, 0x01, 0x01, 'A'}...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeRequestMessage(t *testing.T) {
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
			data: append([]byte{0x09, 0x00, byte(len("trackname"))}, "trackname"...),
			expect: &SubscribeMessage{
				SubscribeID:    9,
				TrackAlias:     0,
				TrackNamespace: "trackname",
			},
			err: io.EOF,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x00),
			expect: &SubscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
			},
			err: errInvalidFilterType,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x00),
			expect: &SubscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				FilterType:     FilterTypeLatestGroup,
				StartGroup:     0,
				StartObject:    0,
				EndGroup:       0,
				EndObject:      0,
				Parameters:     Parameters{},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x03, 0x01, 0x01),
			expect: &SubscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				FilterType:     3,
				StartGroup:     1,
				StartObject:    1,
				EndGroup:       0,
				EndObject:      0,
				Parameters:     map[uint64]Parameter{},
			},
			err: io.EOF,
		},
		{
			data: append(append([]byte{0x011, 0x012, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x01, 0x01, 0x01, 'A'),
			expect: &SubscribeMessage{
				SubscribeID:    17,
				TrackAlias:     18,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				FilterType:     1,
				StartGroup:     0,
				StartObject:    0,
				EndGroup:       0,
				EndObject:      0,
				Parameters:     Parameters{PathParameterKey: &StringParameter{Type: PathParameterKey, Value: "A"}},
			},
			err: nil,
		},
		{
			data: append(append([]byte{0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x04, 0x01, 0x02, 0x03, 0x04, 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			expect: &SubscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				FilterType:     FilterTypeAbsoluteRange,
				StartGroup:     1,
				StartObject:    2,
				EndGroup:       3,
				EndObject:      4,
				Parameters:     Parameters{PathParameterKey: &StringParameter{Type: PathParameterKey, Value: "A"}},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &SubscribeMessage{}
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
