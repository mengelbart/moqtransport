package wire

import (
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
				Type:  MaxSubscribeIDParameterKey,
				Value: uint64(1),
			},
			buf:    nil,
			expect: []byte{0x02, 0x01, 0x01},
		},
		{
			p: VarintParameter{
				Type:  MaxSubscribeIDParameterKey,
				Value: uint64(2),
			},
			buf:    []byte{},
			expect: []byte{0x02, 0x01, 0x02},
		},
		{
			p: VarintParameter{
				Type:  MaxSubscribeIDParameterKey,
				Value: uint64(3),
			},
			buf:    []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02, 0x02, 0x01, 0x03},
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
		pm     map[uint64]parameterParser
		err    error
		n      int
	}{
		{
			data: []byte{byte(MaxSubscribeIDParameterKey), 0x01, 0x01},
			expect: VarintParameter{
				Type:  MaxSubscribeIDParameterKey,
				Name:  "max_subscribe_id",
				Value: uint64(1),
			},
			pm:  setupParameterParsers,
			err: nil,
			n:   3,
		},
		{
			data: append(append([]byte{byte(PathParameterKey)}, quicvarint.Append([]byte{}, uint64(len("/path/param")))...), "/path/param"...),
			expect: StringParameter{
				Type:  1,
				Name:  "path",
				Value: "/path/param",
			},
			pm:  setupParameterParsers,
			err: nil,
			n:   13,
		},
		{
			data:   []byte{},
			expect: nil,
			pm:     versionSpecificParameterParsers,
			err:    io.EOF,
			n:      0,
		},
		{
			data: []byte{0x05, 0x01, 0x00},
			expect: UnknownParameter{
				Type:  5,
				Value: []byte{0x00},
			},
			pm:  versionSpecificParameterParsers,
			err: nil,
			n:   3,
		},
		{
			data: []byte{0x01, 0x01, 'A'},
			expect: StringParameter{
				Type:  PathParameterKey,
				Name:  "path",
				Value: "A",
			},
			pm:  setupParameterParsers,
			err: nil,
			n:   3,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, n, err := parseParameter(tc.data, tc.pm)
			assert.Equal(t, tc.expect, res)
			assert.Equal(t, tc.n, n)
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
			data: []byte{0x01, 0x01, 0x01, 'A'},
			expect: Parameters{PathParameterKey: StringParameter{
				Type:  1,
				Name:  "path",
				Value: "A",
			}},
			err: nil,
		},
		{
			data: []byte{0x02, 0x02, 0x01, 0x03, 0x01, 0x01, 'A'},
			expect: Parameters{
				MaxSubscribeIDParameterKey: VarintParameter{
					Type:  2,
					Name:  "max_subscribe_id",
					Value: uint64(3),
				},
				PathParameterKey: StringParameter{
					Type:  1,
					Name:  "path",
					Value: "A",
				},
			},
			err: nil,
		},
		{
			data: []byte{0x01, 0x01, 0x01, 'A', 0x02, 0x02, 0x02, 0x02},
			expect: Parameters{PathParameterKey: StringParameter{
				Type:  1,
				Name:  "path",
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
			expect: Parameters{
				0x0f: UnknownParameter{
					Type:  0x0f,
					Value: []byte{0x00},
				},
				PathParameterKey: StringParameter{
					Type:  PathParameterKey,
					Name:  "path",
					Value: "A",
				},
			},
			err: nil,
		},
		{
			data: []byte{0x02, 0x02, 0x01, 0x01, 0x02, 0x01, 0x02},
			expect: Parameters{0x02: VarintParameter{
				Type:  2,
				Name:  "max_subscribe_id",
				Value: 1,
			}},
			err: errDuplicateParameter,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := Parameters{}
			err := res.parseSetupParameters(tc.data)
			assert.Equal(t, tc.expect, res)
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
