package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeErrorMessageAppend(t *testing.T) {
	cases := []struct {
		sem    SubscribeErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			sem: SubscribeErrorMessage{
				RequestID:    0,
				ErrorCode:    0,
				ReasonPhrase: "",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: SubscribeErrorMessage{
				RequestID:    17,
				ErrorCode:    12,
				ReasonPhrase: "reason",
			},
			buf:    []byte{},
			expect: []byte{0x11, 0x0c, 0x06, 'r', 'e', 'a', 's', 'o', 'n'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sem.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeErrorMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *SubscribeErrorMessage
		err    error
	}{
		{
			data:   nil,
			expect: &SubscribeErrorMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 0x03, 0x04},
			expect: &SubscribeErrorMessage{
				RequestID:    1,
				ErrorCode:    2,
				ReasonPhrase: "",
			},
			err: io.ErrUnexpectedEOF,
		},
		{
			data: []byte{0x00, 0x01, 0x05, 'e', 'r', 'r', 'o', 'r'},
			expect: &SubscribeErrorMessage{
				RequestID:    0,
				ErrorCode:    1,
				ReasonPhrase: "error",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &SubscribeErrorMessage{}
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
