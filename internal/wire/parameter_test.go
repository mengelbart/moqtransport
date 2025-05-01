package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseParameters(t *testing.T) {
	cases := []struct {
		data   []byte
		expect Parameters
		err    error
	}{
		{
			data:   nil,
			expect: Parameters{},
			err:    io.EOF,
		},
		{
			data:   nil,
			expect: Parameters{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: Parameters{},
			err:    io.EOF,
		},
		{
			data: []byte{0x01, 0x01, 0x01, 'A'},
			expect: Parameters{KeyValuePair{
				Type:       1,
				ValueBytes: []byte("A"),
			}},
			err: nil,
		},
		{
			data: []byte{0x02, 0x02, 0x03, 0x01, 0x01, 'A'},
			expect: Parameters{
				KeyValuePair{
					Type:        2,
					ValueVarInt: uint64(3),
				},
				KeyValuePair{
					Type:       1,
					ValueBytes: []byte("A"),
				},
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x01, 0x01, 'A', 0x02, 0x02, 0x02, 0x02},
			expect: Parameters{KeyValuePair{
				Type:       1,
				ValueBytes: []byte("A"),
			}},
			err: nil,
		},
		{
			data:   []byte{},
			expect: Parameters{},
			err:    io.EOF,
		},
		{
			data: []byte{0x02, 0x0f, 0x01, 0x00, 0x01, 0x01, 'A'},
			expect: Parameters{
				KeyValuePair{
					Type:       0x0f,
					ValueBytes: []byte{0x00},
				},
				KeyValuePair{
					Type:       PathParameterKey,
					ValueBytes: []byte("A"),
				},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := Parameters{}
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
