package wire

import (
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
				RequestID:    0,
				StatusCode:   0,
				StreamCount:  0,
				ReasonPhrase: "",
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			srm: SubscribeDoneMessage{
				RequestID:    0,
				StatusCode:   1,
				StreamCount:  2,
				ReasonPhrase: "reason",
			},
			buf: []byte{},
			expect: []byte{
				0x00,
				0x01,
				0x02,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
			},
		},
		{
			srm: SubscribeDoneMessage{
				RequestID:    17,
				StatusCode:   1,
				StreamCount:  4,
				ReasonPhrase: "reason",
			},
			buf: []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{
				0x0a, 0x0b, 0x0c, 0x0d,
				0x11,
				0x01,
				0x04,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
			},
		},
		{
			srm: SubscribeDoneMessage{
				RequestID:    0,
				StatusCode:   0,
				StreamCount:  0,
				ReasonPhrase: "",
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			srm: SubscribeDoneMessage{
				RequestID:    0,
				StatusCode:   1,
				StreamCount:  2,
				ReasonPhrase: "reason",
			},
			buf: []byte{},
			expect: []byte{
				0x00,
				0x01,
				0x02,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
			},
		},
		{
			srm: SubscribeDoneMessage{
				RequestID:    17,
				StatusCode:   1,
				StreamCount:  2,
				ReasonPhrase: "reason",
			},
			buf: []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{
				0x0a, 0x0b, 0x0c, 0x0d,
				0x11,
				0x01,
				0x02,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
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
				0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00,
			},
			expect: &SubscribeDoneMessage{
				RequestID:    0,
				StatusCode:   0,
				StreamCount:  0,
				ReasonPhrase: "",
			},
			err: nil,
		},
		{
			data: []byte{
				0x00,
				0x01,
				0x02,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x01,
				0x02,
				0x03,
			},
			expect: &SubscribeDoneMessage{
				RequestID:    0,
				StatusCode:   1,
				StreamCount:  2,
				ReasonPhrase: "reason",
			},
			err: nil,
		},
		{
			data: []byte{
				0x00,
				0x01,
				0x02,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x00,
			},
			expect: &SubscribeDoneMessage{
				RequestID:    0,
				StatusCode:   1,
				StreamCount:  2,
				ReasonPhrase: "reason",
			},
			err: nil,
		},
		{
			data: []byte{
				0x00, 0x00, 0x00, 0x00,
			},
			expect: &SubscribeDoneMessage{
				RequestID:    0,
				StatusCode:   0,
				StreamCount:  0,
				ReasonPhrase: "",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &SubscribeDoneMessage{}
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
