package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnsubscribeMessageAppend(t *testing.T) {
	cases := []struct {
		usm    UnsubscribeMessage
		buf    []byte
		expect []byte
	}{
		{
			usm: UnsubscribeMessage{
				RequestID: 17,
			},
			buf: []byte{},
			expect: []byte{
				0x11,
			},
		},
		{
			usm: UnsubscribeMessage{
				RequestID: 17,
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x11},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.usm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseUnsubscribeMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *UnsubscribeMessage
		err    error
	}{
		{
			data:   nil,
			expect: &UnsubscribeMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{17},
			expect: &UnsubscribeMessage{
				RequestID: 17,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &UnsubscribeMessage{}
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
