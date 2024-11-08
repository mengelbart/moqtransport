package wire

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientSetupMessageAppend(t *testing.T) {
	cases := []struct {
		csm    ClientSetupMessage
		buf    []byte
		expect []byte
	}{
		{
			csm: ClientSetupMessage{
				SupportedVersions: nil,
				SetupParameters:   nil,
			},
			buf: []byte{},
			expect: []byte{
				0x00, 0x00,
			},
		},
		{
			csm: ClientSetupMessage{
				SupportedVersions: []Version{Draft_ietf_moq_transport_00},
				SetupParameters:   Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				0x01, 0xc0, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			csm: ClientSetupMessage{
				SupportedVersions: []Version{Draft_ietf_moq_transport_00},
				SetupParameters: Parameters{PathParameterKey: StringParameter{
					Type:  PathParameterKey,
					Value: "A",
				}},
			},
			buf: []byte{},
			expect: []byte{
				0x01, 0xc0, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A',
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.csm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseClientSetupMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *ClientSetupMessage
		err    error
	}{
		{
			data:   nil,
			expect: &ClientSetupMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &ClientSetupMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{
				0x01, 0x00,
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0x00},
				SetupParameters:   Parameters{},
			},
			err: io.EOF,
		},
		{
			data: []byte{
				0x01,
			},
			expect: &ClientSetupMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{
				0x02, 0x00, 0x00 + 1,
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0x00, 0x01},
				SetupParameters:   Parameters{},
			},
			err: io.EOF,
		},
		{
			data: []byte{
				0x02, 0x00, 0x00 + 1, 0x00,
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0, 0 + 1},
				SetupParameters:   Parameters{},
			},
			err: nil,
		},
		{
			data: []byte{
				0x01, 0x00, 0x00,
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0},
				SetupParameters:   Parameters{},
			},
			err: nil,
		},
		{
			data: []byte{
				0x01, 0x00,
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0x00},
				SetupParameters:   Parameters{},
			},
			err: io.EOF,
		},
		{
			data: []byte{
				0x01, 0x00,
				0x02, 0x00, 0x01, 0x02, 0x01, 0x01, 'a',
			},
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{0x00},
				SetupParameters: Parameters{
					0x00: VarintParameter{
						Type:  0,
						Value: 2,
					},
					0x01: StringParameter{
						Type:  1,
						Value: "a",
					},
				},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := &ClientSetupMessage{}
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
