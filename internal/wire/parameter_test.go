package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/quic-go/quic-go/quicvarint"
	"github.com/stretchr/testify/assert"
)

func TestParameterAppend(t *testing.T) {
	cases := []struct {
		p      Parameter
		buf    []byte
		expect []byte
	}{
		{
			p: StringParameter{
				Type:  1,
				Value: "",
			},
			buf:    nil,
			expect: []byte{0x01, 0x00},
		},
		{
			p: StringParameter{
				Type:  1,
				Value: "A",
			},
			buf:    nil,
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p: StringParameter{
				Type:  1,
				Value: "A",
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p: StringParameter{
				Type:  1,
				Value: "A",
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x01, 0x01, 'A'},
		},
		{
			p: VarintParameter{
				Type:  0,
				Value: uint64(RolePublisher),
			},
			buf:    nil,
			expect: []byte{0x00, 0x01, 0x01},
		},
		{
			p: VarintParameter{
				Type:  0,
				Value: uint64(RoleSubscriber),
			},
			buf:    nil,
			expect: []byte{0x00, 0x01, 0x02},
		},
		{
			p: VarintParameter{
				Type:  0,
				Value: uint64(RolePubSub),
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x01, 0x03},
		},
		{
			p: VarintParameter{
				Type:  0,
				Value: uint64(RolePubSub),
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x00, 0x01, 0x03},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.p.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseParameter(t *testing.T) {
	cases := []struct {
		data   []byte
		expect Parameter
		err    error
	}{
		{
			data: []byte{byte(RoleParameterKey), 0x01, byte(RolePublisher)},
			expect: &VarintParameter{
				Type:  0,
				Value: uint64(RolePublisher),
			},
			err: nil,
		},
		{
			data: append(append([]byte{byte(PathParameterKey)}, quicvarint.Append([]byte{}, uint64(len("/path/param")))...), "/path/param"...),
			expect: &StringParameter{
				Type:  1,
				Value: "/path/param",
			},
			err: nil,
		},
		{
			data:   []byte{},
			expect: nil,
			err:    io.EOF,
		},
		{
			data:   []byte{0x05, 0x01, 0x00},
			expect: nil,
			err:    nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res, err := parseParameter(reader)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

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
			data:   []byte{0x01, 0x01, 0x01, 'A'},
			expect: Parameters{PathParameterKey: &StringParameter{Type: 1, Value: "A"}},
			err:    nil,
		},
		{
			data: []byte{0x02, 0x00, 0x01, 0x01, 0x01, 0x01, 'A'},
			expect: Parameters{
				RoleParameterKey: &VarintParameter{
					Type:  0,
					Value: uint64(RolePublisher),
				},
				PathParameterKey: &StringParameter{
					Type:  1,
					Value: "A",
				},
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x01, 0x01, 'A', 0x02, 0x02, 0x02, 0x02},
			expect: Parameters{PathParameterKey: &StringParameter{
				Type:  1,
				Value: "A",
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
			expect: Parameters{PathParameterKey: &StringParameter{
				Type:  PathParameterKey,
				Value: "A",
			}},
			err: nil,
		},
		{
			data:   []byte{0x02, 0x00, 0x01, 0x01, 0x00, 0x01, 0x02},
			expect: Parameters{0x00: &VarintParameter{0, 1}},
			err:    errDuplicateParameter,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := Parameters{}
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
