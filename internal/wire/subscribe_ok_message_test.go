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
				SubscribeID:     0,
				Expires:         0,
				GroupOrder:      1,
				ContentExists:   true,
				LargestGroupID:  1,
				LargestObjectID: 2,
				Parameters:      map[uint64]Parameter{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00, 0x01, 0x01, 0x01, 0x02, 0x00,
			},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:     17,
				Expires:         1000,
				GroupOrder:      1,
				ContentExists:   true,
				LargestGroupID:  1,
				LargestObjectID: 2,
				Parameters:      map[uint64]Parameter{},
			},
			buf:    []byte{},
			expect: []byte{0x11, 0x43, 0xe8, 0x01, 0x01, 0x01, 0x02, 0x00},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:     17,
				Expires:         1000,
				GroupOrder:      2,
				ContentExists:   true,
				LargestGroupID:  1,
				LargestObjectID: 2,
				Parameters:      map[uint64]Parameter{},
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, 0x11, 0x43, 0xe8, 0x02, 0x01, 0x01, 0x02, 0x00},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:     0,
				Expires:         0,
				GroupOrder:      2,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      map[uint64]Parameter{},
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00, 0x02, 0x00, 0x00,
			},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:     17,
				Expires:         1000,
				GroupOrder:      1,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      map[uint64]Parameter{},
			},
			buf:    []byte{},
			expect: []byte{0x11, 0x43, 0xe8, 0x01, 0x00, 0x00},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:     17,
				Expires:         1000,
				GroupOrder:      2,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      map[uint64]Parameter{},
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, 0x11, 0x43, 0xe8, 0x02, 0x00, 0x00},
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
			data: []byte{0x01, 0x10, 0x01, 0x00, 0x00},
			expect: &SubscribeOkMessage{
				SubscribeID:     1,
				Expires:         0x10 * time.Millisecond,
				GroupOrder:      1,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      map[uint64]Parameter{},
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x10, 0x02, 0x01, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				SubscribeID:     1,
				Expires:         0x10 * time.Millisecond,
				GroupOrder:      2,
				ContentExists:   true,
				LargestGroupID:  1,
				LargestObjectID: 2,
				Parameters:      map[uint64]Parameter{},
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x10, 0x02, 0x08, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				SubscribeID:     1,
				Expires:         0x10 * time.Millisecond,
				GroupOrder:      2,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      nil,
			},
			err: errInvalidContentExistsByte,
		},
		{
			data: []byte{0x01, 0x10, 0x09, 0x08, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				SubscribeID: 1,
				Expires:     0x10 * time.Millisecond,
				GroupOrder:  9,
			},
			err: errInvalidGroupOrder,
		},
		{
			data: []byte{0x01, 0x10, 0x03, 0x08, 0x01, 0x02, 0x00},
			expect: &SubscribeOkMessage{
				SubscribeID: 1,
				Expires:     0x10 * time.Millisecond,
				GroupOrder:  3,
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
