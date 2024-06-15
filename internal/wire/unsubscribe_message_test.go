package wire

import (
	"bufio"
	"bytes"
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
				SubscribeID: 17,
			},
			buf: []byte{},
			expect: []byte{
				byte(unsubscribeMessageType), 0x11,
			},
		},
		{
			usm: UnsubscribeMessage{
				SubscribeID: 17,
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(unsubscribeMessageType), 0x11},
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
				SubscribeID: 17,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &UnsubscribeMessage{}
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
