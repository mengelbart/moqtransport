package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectMessageAppend(t *testing.T) {
	cases := []struct {
		om     ObjectMessage
		buf    []byte
		expect []byte
	}{
		{
			om: ObjectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(ObjectStreamMessageType), 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: ObjectMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				GroupID:         3,
				ObjectID:        4,
				ObjectSendOrder: 5,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(ObjectStreamMessageType), 0x01, 0x02, 0x03, 0x04, 0x05,
				0x01, 0x02, 0x03,
			},
		},
		{
			om: ObjectMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				GroupID:         3,
				ObjectID:        4,
				ObjectSendOrder: 5,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{0x01, 0x02, 0x03},
			expect: []byte{
				0x01, 0x02, 0x03,
				byte(ObjectStreamMessageType), 0x01, 0x02, 0x03, 0x04, 0x05,
				0x01, 0x02, 0x03,
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.om.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseObjectMessage(t *testing.T) {
	cases := []struct {
		data      []byte
		expect    *ObjectMessage
		expectedN int
		err       error
	}{
		{
			data:      nil,
			expect:    &ObjectMessage{},
			expectedN: 0,
			err:       io.EOF,
		},
		{
			data:      []byte{},
			expect:    &ObjectMessage{},
			expectedN: 0,
			err:       io.EOF,
		},
		{
			data:      []byte{0x00, 0x00, 0x00},
			expect:    &ObjectMessage{},
			expectedN: 3,
			err:       io.EOF,
		},
		{
			data:      []byte{0x02, 0x00, 0x00},
			expect:    &ObjectMessage{SubscribeID: 0x02},
			expectedN: 3,
			err:       io.EOF,
		},
		{
			data: []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			expect: &ObjectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			expectedN: 5,
			err:       nil,
		},
		{
			data: []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			expect: &ObjectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			expectedN: 5,
			err:       nil,
		},
		{
			data: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d},
			expect: &ObjectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			expectedN: 9,
			err:       nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &ObjectMessage{}
			n, err := res.parse(tc.data)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, tc.expectedN, n)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
