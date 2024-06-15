package wire

import (
	"bufio"
	"bytes"
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
				SubscribeID:   0,
				Expires:       0,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeOkMessageType), 0x00, 0x00, 0x01, 0x01, 0x02,
			},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			buf:    []byte{},
			expect: []byte{byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x01, 0x01, 0x02},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x01, 0x01, 0x02},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:   0,
				Expires:       0,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeOkMessageType), 0x00, 0x00, 0x00,
			},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{},
			expect: []byte{byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x00},
		},
		{
			som: SubscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x00},
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
			data: []byte{0x01, 0x10, 0x00},
			expect: &SubscribeOkMessage{
				SubscribeID:   1,
				Expires:       0x10 * time.Millisecond,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x10, 0x01, 0x01, 0x02},
			expect: &SubscribeOkMessage{
				SubscribeID:   1,
				Expires:       0x10 * time.Millisecond,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x10, 0x08, 0x01, 0x02},
			expect: &SubscribeOkMessage{
				SubscribeID:   1,
				Expires:       0x10 * time.Millisecond,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: errInvalidContentExistsByte,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &SubscribeOkMessage{}
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
