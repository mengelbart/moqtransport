package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamHeaderTrackObjectAppend(t *testing.T) {
	cases := []struct {
		shto   StreamHeaderTrackObject
		buf    []byte
		expect []byte
	}{
		{
			shto: StreamHeaderTrackObject{
				GroupID:       0,
				ObjectID:      0,
				ObjectStatus:  0,
				ObjectPayload: []byte{},
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			shto: StreamHeaderTrackObject{
				GroupID:       0,
				ObjectID:      1,
				ObjectStatus:  0,
				ObjectPayload: []byte{0x00, 0x01, 0x02},
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x01, 0x03, 0x00, 0x01, 0x02},
		},
		{
			shto: StreamHeaderTrackObject{
				GroupID:       1,
				ObjectID:      2,
				ObjectPayload: []byte{0x01, 0x02},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, 0x01, 0x02, 0x02, 0x01, 0x02},
		},
		{
			shto: StreamHeaderTrackObject{
				GroupID:       1,
				ObjectID:      2,
				ObjectStatus:  ObjectStatusEndOfGroup,
				ObjectPayload: []byte{},
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x02, 0x00, 0x03},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.shto.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseStreamHeaderTrackObjectAppend(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *StreamHeaderTrackObject
		err    error
	}{
		{
			data:   nil,
			expect: &StreamHeaderTrackObject{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &StreamHeaderTrackObject{},
			err:    io.EOF,
		},
		{
			data: []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			expect: &StreamHeaderTrackObject{
				GroupID:       0,
				ObjectID:      1,
				ObjectPayload: []byte{0x03, 0x04},
			},
			err: nil,
		},
		{
			data: []byte{0x00, 0x01, 0x00, 0x03},
			expect: &StreamHeaderTrackObject{
				GroupID:       0,
				ObjectID:      1,
				ObjectStatus:  ObjectStatusEndOfGroup,
				ObjectPayload: nil,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &StreamHeaderTrackObject{}
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
