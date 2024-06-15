package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamHeaderGroupObjectAppend(t *testing.T) {
	cases := []struct {
		shgo   StreamHeaderGroupObject
		buf    []byte
		expect []byte
	}{
		{
			shgo: StreamHeaderGroupObject{
				ObjectID:      0,
				ObjectPayload: []byte{},
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x00},
		},
		{
			shgo: StreamHeaderGroupObject{
				ObjectID:      1,
				ObjectPayload: []byte{0x01, 0x02},
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x02, 0x01, 0x02},
		},
		{
			shgo: StreamHeaderGroupObject{
				ObjectID:      2,
				ObjectPayload: []byte{0x01, 0x02},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x02, 0x02, 0x01, 0x02},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.shgo.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseStreamHeaderGroupObject(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *StreamHeaderGroupObject
		err    error
	}{
		{
			data:   nil,
			expect: &StreamHeaderGroupObject{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &StreamHeaderGroupObject{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x02, 0x03, 0x04},
			expect: &StreamHeaderGroupObject{
				ObjectID:      1,
				ObjectPayload: []byte{0x03, 0x04},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &StreamHeaderGroupObject{}
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
