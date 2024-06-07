package moqtransport

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/quic-go/quic-go/quicvarint"
	"github.com/stretchr/testify/assert"
)

func TestParameterAppend(t *testing.T) {
	cases := []struct {
		p      parameter
		buf    []byte
		expect []byte
	}{
		{
			p: stringParameter{
				K: 1,
				V: "",
			},
			buf:    nil,
			expect: []byte{0x01, 0x00},
		},
		{
			p: stringParameter{
				K: 1,
				V: "A",
			},
			buf:    nil,
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p: stringParameter{
				K: 1,
				V: "A",
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x01, 'A'},
		},
		{
			p: stringParameter{
				K: 1,
				V: "A",
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x01, 0x01, 'A'},
		},
		{
			p: varintParameter{
				K: 0,
				V: uint64(RolePublisher),
			},
			buf:    nil,
			expect: []byte{0x00, 0x01, 0x01},
		},
		{
			p: varintParameter{
				K: 0,
				V: uint64(RoleSubscriber),
			},
			buf:    nil,
			expect: []byte{0x00, 0x01, 0x02},
		},
		{
			p: varintParameter{
				K: 0,
				V: uint64(RolePubSub),
			},
			buf:    []byte{},
			expect: []byte{0x00, 0x01, 0x03},
		},
		{
			p: varintParameter{
				K: 0,
				V: uint64(RolePubSub),
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
		buf    []byte
		expect parameter
		err    error
	}{
		{
			buf: []byte{byte(roleParameterKey), 0x01, byte(RolePublisher)},
			expect: varintParameter{
				K: 0,
				V: uint64(RolePublisher),
			},
			err: nil,
		},
		{
			buf: append(append([]byte{byte(pathParameterKey)}, quicvarint.Append([]byte{}, uint64(len("/path/param")))...), "/path/param"...),
			expect: stringParameter{
				K: 1,
				V: "/path/param",
			},
			err: nil,
		},
		{
			buf:    []byte{},
			expect: nil,
			err:    io.EOF,
		},
		{
			buf:    []byte{0x05, 0x01, 0x00},
			expect: nil,
			err:    nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			r := bytes.NewReader(tc.buf)
			res, err := parseParameter(r)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				assert.Equal(t, tc.expect, res)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, res)
			assert.Zero(t, r.Len())
		})
	}
}

func TestParseParameters(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect parameters
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader(nil),
			expect: nil,
			err:    io.EOF,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{0x01, 0x01, 0x01, 'A'}),
			expect: parameters{
				pathParameterKey: stringParameter{
					K: 1,
					V: "A",
				},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{0x02, 0x00, 0x01, 0x01, 0x01, 0x01, 'A'}),
			expect: parameters{
				roleParameterKey: varintParameter{
					K: 0,
					V: uint64(RolePublisher),
				},
				pathParameterKey: stringParameter{
					K: 1,
					V: "A",
				},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{0x01, 0x01, 0x01, 'A', 0x02, 0x02, 0x02, 0x02}),
			expect: parameters{pathParameterKey: stringParameter{
				K: 1,
				V: "A",
			}},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{0x02, 0x0f, 0x01, 0x00, 0x01, 0x01, 'A'}),
			expect: parameters{pathParameterKey: stringParameter{
				K: pathParameterKey,
				V: "A",
			}},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x02, 0x00, 0x01, 0x01, 0x00, 0x01, 0x02}),
			expect: nil,
			err:    errDuplicateParameter,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseParameters(tc.r)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				assert.Equal(t, tc.expect, res)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expect, res)
		})
	}
}
