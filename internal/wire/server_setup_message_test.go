package wire

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerSetupMessageAppend(t *testing.T) {
	cases := []struct {
		ssm    ServerSetupMessage
		buf    []byte
		expect []byte
	}{
		{
			ssm: ServerSetupMessage{
				SelectedVersion: 0,
				SetupParameters: nil,
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(serverSetupMessageType), 0x00, 0x00,
			},
		},
		{
			ssm: ServerSetupMessage{
				SelectedVersion: 0,
				SetupParameters: Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(serverSetupMessageType), 0x00, 0x00,
			},
		},
		{
			ssm: ServerSetupMessage{
				SelectedVersion: 0,
				SetupParameters: Parameters{RoleParameterKey: VarintParameter{
					Type:  RoleParameterKey,
					Value: uint64(RolePublisher),
				}},
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(serverSetupMessageType), 0x00, 0x01, 0x00, 0x01, 0x01,
			},
		},
		{
			ssm: ServerSetupMessage{
				SelectedVersion: 0,
				SetupParameters: Parameters{PathParameterKey: StringParameter{
					Type:  PathParameterKey,
					Value: "A",
				}},
			},
			buf: []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02,
				0x40, byte(serverSetupMessageType), 0x00, 0x01, 0x01, 0x01, 'A',
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.ssm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseServerSetupMessage(t *testing.T) {
	cases := []struct {
		data   []byte
		expect *ServerSetupMessage
		err    error
	}{
		{
			data:   nil,
			expect: &ServerSetupMessage{},
			err:    io.EOF,
		},
		{
			data:   []byte{},
			expect: &ServerSetupMessage{},
			err:    io.EOF,
		},
		{
			data: []byte{
				0x00, 0x01,
			},
			expect: &ServerSetupMessage{
				SelectedVersion: 0,
				SetupParameters: map[uint64]Parameter{},
			},
			err: io.EOF,
		},
		{
			data: []byte{
				0xc0, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00,
			},
			expect: &ServerSetupMessage{
				SelectedVersion: Draft_ietf_moq_transport_00,
				SetupParameters: Parameters{},
			},
			err: nil,
		},
		{
			data: []byte{
				0x00, 0x01, 0x01, 0x01, 'A',
			},
			expect: &ServerSetupMessage{
				SelectedVersion: 0,
				SetupParameters: Parameters{PathParameterKey: &StringParameter{
					Type:  PathParameterKey,
					Value: "A",
				}},
			},
			err: nil,
		},
		{
			data: []byte{
				0x00, 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c, 0x0d,
			},
			expect: &ServerSetupMessage{
				SelectedVersion: 0,
				SetupParameters: Parameters{PathParameterKey: &StringParameter{
					Type:  PathParameterKey,
					Value: "A",
				}},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tc.data))
			res := &ServerSetupMessage{}
			err := res.parse(reader)
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
