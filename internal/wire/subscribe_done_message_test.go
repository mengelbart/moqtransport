package wire

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeDoneMessageAppend(t *testing.T) {
	cases := []struct {
		srm    SubscribeDoneMessage
		buf    []byte
		expect []byte
	}{
		{
			srm: SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			srm: SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: false,
				FinalGroup:    2,
				FinalObject:   3,
			},
			buf: []byte{},
			expect: []byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x00,
			},
		},
		{
			srm: SubscribeDoneMessage{
				SubscribeID:   17,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: false,
			},
			buf: []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{
				0x0a, 0x0b, 0x0c, 0x0d,
				0x11,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x00,
			},
		},
		{
			srm: SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: true,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00},
		},
		{
			srm: SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: true,
				FinalGroup:    2,
				FinalObject:   3,
			},
			buf: []byte{},
			expect: []byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x01,
				0x02,
				0x03,
			},
		},
		{
			srm: SubscribeDoneMessage{
				SubscribeID:   17,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: true,
				FinalGroup:    2,
				FinalObject:   3,
			},
			buf: []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{
				0x0a, 0x0b, 0x0c, 0x0d,
				0x11,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x01,
				0x02,
				0x03,
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.srm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeDoneMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *SubscribeDoneMessage
		err    error
	}{
		{
			data:   nil,
			expect: &SubscribeDoneMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &SubscribeDoneMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{
				0x00, 0x00, 0x00, 0x01, 0x00, 0x00,
			},
			expect: &SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: true,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: nil,
		},
		{
			data: []byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x01,
				0x02,
				0x03,
			},
			expect: &SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: true,
				FinalGroup:    2,
				FinalObject:   3,
			},
			err: nil,
		},
		{
			data: []byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x00,
			},
			expect: &SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: false,
			},
			err: nil,
		},
		{
			data: []byte{
				0x00, 0x00, 0x00, 0x00,
			},
			expect: &SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: nil,
		},
		{
			data: []byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x07,
			},
			expect: &SubscribeDoneMessage{
				SubscribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: errors.New("invalid use of ContentExists byte"),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &SubscribeDoneMessage{}
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
