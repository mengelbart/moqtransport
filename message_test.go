package moqtransport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestObjectStreamMessageAppend(t *testing.T) {
	cases := []struct {
		om     objectMessage
		buf    []byte
		expect []byte
	}{
		{
			om: objectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(objectStreamMessageType), 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: objectMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				GroupID:         3,
				ObjectID:        4,
				ObjectSendOrder: 5,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(objectStreamMessageType), 0x01, 0x02, 0x03, 0x04, 0x05,
				0x01, 0x02, 0x03,
			},
		},
		{
			om: objectMessage{
				SubscribeID:     1,
				TrackAlias:      2,
				GroupID:         3,
				ObjectID:        4,
				ObjectSendOrder: 5,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{0x01, 0x02, 0x03},
			expect: []byte{
				0x01, 0x02, 0x03,
				byte(objectStreamMessageType), 0x01, 0x02, 0x03, 0x04, 0x05,
				0x01, 0x02, 0x03,
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.om.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseObjectMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *objectMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r:      bytes.NewReader([]byte{0x02, 0x00, 0x00}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00}),
			expect: &objectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00}),
			expect: &objectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			expect: &objectMessage{
				SubscribeID:     0,
				TrackAlias:      0,
				GroupID:         0,
				ObjectID:        0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseObjectMessage(objectStreamMessageType)
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

func TestClientSetupMessageAppend(t *testing.T) {
	cases := []struct {
		csm    clientSetupMessage
		buf    []byte
		expect []byte
	}{
		{
			csm: clientSetupMessage{
				SupportedVersions: nil,
				SetupParameters:   nil,
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(clientSetupMessageType), 0x00, 0x00,
			},
		},
		{
			csm: clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters:   parameters{},
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(clientSetupMessageType), 0x01, 0xc0, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			csm: clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(clientSetupMessageType), 0x01, 0xc0, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A',
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.csm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseClientSetupMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *clientSetupMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, 0x00,
			}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x01,
			}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x02, 0x00, 0x00 + 1,
			}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x02, 0x00, 0x00 + 1, 0x00,
			}),
			expect: &clientSetupMessage{
				SupportedVersions: []version{0, 0 + 1},
				SetupParameters:   parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, 0x00, 0x00,
			}),
			expect: &clientSetupMessage{
				SupportedVersions: []version{0},
				SetupParameters:   parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, 0x00,
			}),
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseClientSetupMessage()
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

func TestServerSetupMessageAppend(t *testing.T) {
	cases := []struct {
		ssm    serverSetupMessage
		buf    []byte
		expect []byte
	}{
		{
			ssm: serverSetupMessage{
				SelectedVersion: 0,
				SetupParameters: nil,
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(serverSetupMessageType), 0x00, 0x00,
			},
		},
		{
			ssm: serverSetupMessage{
				SelectedVersion: 0,
				SetupParameters: parameters{},
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(serverSetupMessageType), 0x00, 0x00,
			},
		},
		{
			ssm: serverSetupMessage{
				SelectedVersion: 0,
				SetupParameters: parameters{roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(IngestionRole),
				}},
			},
			buf: []byte{},
			expect: []byte{
				0x40, byte(serverSetupMessageType), 0x00, 0x01, 0x00, 0x01, 0x01,
			},
		},
		{
			ssm: serverSetupMessage{
				SelectedVersion: 0,
				SetupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
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
			res := tc.ssm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseServerSetupMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *serverSetupMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x00, 0x01,
			}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0xc0, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00,
			}),
			expect: &serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x00, 0x01, 0x01, 0x01, 'A',
			}),
			expect: &serverSetupMessage{
				SelectedVersion: 0,
				SetupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x00, 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c, 0x0d,
			}),
			expect: &serverSetupMessage{
				SelectedVersion: 0,
				SetupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseServerSetupMessage()
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

func TestLocationAppend(t *testing.T) {
	cases := []struct {
		loc    Location
		buf    []byte
		expect []byte
	}{
		{
			loc:    Location{},
			buf:    []byte{},
			expect: []byte{0x00},
		},
		{
			loc: Location{
				Mode:  1,
				Value: 0,
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x00},
		},
		{
			loc: Location{
				Mode:  2,
				Value: 10,
			},
			buf:    []byte{},
			expect: []byte{0x02, 0x0A},
		},
		{
			loc: Location{
				Mode:  3,
				Value: 8,
			},
			buf:    []byte{0x0A, 0x0B},
			expect: []byte{0x0A, 0x0B, 0x03, 0x08},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.loc.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseLocation(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect Location
		err    error
	}{
		{
			r:      nil,
			expect: Location{},
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: Location{},
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{0x00}),
			expect: Location{
				Mode:  0,
				Value: 0,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{0x01, 0x02}),
			expect: Location{
				Mode:  1,
				Value: 2,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseLocation()
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

func TestSubscribeRequestMessageAppend(t *testing.T) {
	cases := []struct {
		srm    subscribeMessage
		buf    []byte
		expect []byte
	}{
		{
			srm: subscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "",
				TrackName:      "",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeMessageType), 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0, 0x00,
			},
		},
		{
			srm: subscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     Location{Mode: 0, Value: 0},
				StartObject:    Location{Mode: 1, Value: 0},
				EndGroup:       Location{Mode: 2, Value: 0},
				EndObject:      Location{Mode: 3, Value: 0},
				Parameters:     parameters{},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeMessageType), 0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x00),
		},
		{
			srm: subscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     parameters{pathParameterKey: stringParameter{k: pathParameterKey, v: "A"}},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeMessageType), 0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A'}...),
		},
		{
			srm: subscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     parameters{pathParameterKey: stringParameter{k: pathParameterKey, v: "A"}},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, byte(subscribeMessageType), 0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A'}...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.srm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeRequestMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *subscribeMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				append([]byte{0x09}, "trackname"...),
			),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x00, 0x00, 0x00, 0x00),
			),
			expect: &subscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x01, 0x01),
			),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x011, 0x012, 0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x01, 0x01, 'A'),
			),
			expect: &subscribeMessage{
				SubscribeID:    17,
				TrackAlias:     18,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     Location{Mode: 1, Value: 2},
				StartObject:    Location{Mode: 1, Value: 2},
				EndGroup:       Location{Mode: 1, Value: 2},
				EndObject:      Location{Mode: 1, Value: 2},
				Parameters:     parameters{pathParameterKey: stringParameter{k: pathParameterKey, v: "A"}},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x00, 0x00, 0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			),
			expect: &subscribeMessage{
				SubscribeID:    0,
				TrackAlias:     0,
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     parameters{pathParameterKey: stringParameter{k: pathParameterKey, v: "A"}},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append([]byte{0x09}, "trackname"...),
			),
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseSubscribeMessage()
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

func TestSubscribeOkMessageAppend(t *testing.T) {
	cases := []struct {
		som    subscribeOkMessage
		buf    []byte
		expect []byte
	}{
		{
			som: subscribeOkMessage{
				SubscribeID:   0,
				Expires:       0,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeOkMessageType), 0x00, 0x00, 0x01, 0x01, 0x02,
			},
		},
		{
			som: subscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			buf:    []byte{},
			expect: []byte{byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x01, 0x01, 0x02},
		},
		{
			som: subscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x01, 0x01, 0x02},
		},
		{
			som: subscribeOkMessage{
				SubscribeID:   0,
				Expires:       0,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeOkMessageType), 0x00, 0x00, 0x00,
			},
		},
		{
			som: subscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{},
			expect: []byte{byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x00},
		},
		{
			som: subscribeOkMessage{
				SubscribeID:   17,
				Expires:       1000,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, byte(subscribeOkMessageType), 0x11, 0x43, 0xe8, 0x00},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.som.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeOkMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *subscribeOkMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				[]byte{0x01, 0x10, 0x00},
			),
			expect: &subscribeOkMessage{
				SubscribeID:   1,
				Expires:       0x10 * time.Millisecond,
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				[]byte{0x01, 0x10, 0x01, 0x01, 0x02},
			),
			expect: &subscribeOkMessage{
				SubscribeID:   1,
				Expires:       0x10 * time.Millisecond,
				ContentExists: true,
				FinalGroup:    1,
				FinalObject:   2,
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				[]byte{0x01, 0x10, 0x08, 0x01, 0x02},
			),
			expect: nil,
			err:    errors.New("invalid use of ContentExists byte"),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseSubscribeOkMessage()
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

func TestSubscribeErrorMessageAppend(t *testing.T) {
	cases := []struct {
		sem    subscribeErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			sem: subscribeErrorMessage{
				SubscribeID:  0,
				ErrorCode:    0,
				ReasonPhrase: "",
				TrackAlias:   0,
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(subscribeErrorMessageType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				SubscribeID:  17,
				ErrorCode:    12,
				ReasonPhrase: "reason",
				TrackAlias:   0,
			},
			buf:    []byte{},
			expect: []byte{byte(subscribeErrorMessageType), 0x11, 0x0c, 0x06, 'r', 'e', 'a', 's', 'o', 'n', 0x00},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sem.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeErrorMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *subscribeErrorMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x02, 0x03, 0x04}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				[]byte{0x00, 0x01, 0x05, 'e', 'r', 'r', 'o', 'r', 0x11},
			),
			expect: &subscribeErrorMessage{
				SubscribeID:  0,
				ErrorCode:    1,
				ReasonPhrase: "error",
				TrackAlias:   17,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseSubscribeErrorMessage()
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

func TestUnsubscribeMessageAppend(t *testing.T) {
	cases := []struct {
		usm    unsubscribeMessage
		buf    []byte
		expect []byte
	}{
		{
			usm: unsubscribeMessage{
				SubscribeID: 17,
			},
			buf: []byte{},
			expect: []byte{
				byte(unsubscribeMessageType), 0x11,
			},
		},
		{
			usm: unsubscribeMessage{
				SubscribeID: 17,
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(unsubscribeMessageType), 0x11},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.usm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseUnsubscribeMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *unsubscribeMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r: bytes.NewReader([]byte{17}),
			expect: &unsubscribeMessage{
				SubscribeID: 17,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseUnsubscribeMessage()
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

func TestSubscribeDoneMessageAppend(t *testing.T) {
	cases := []struct {
		srm    subscribeDoneMessage
		buf    []byte
		expect []byte
	}{
		{
			srm: subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{},
			expect: []byte{0x0b, 0x00, 0x00, 0x00, 0x00},
		},
		{
			srm: subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: false,
				FinalGroup:    2,
				FinalObject:   3,
			},
			buf: []byte{},
			expect: []byte{
				0x0b,
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x00,
			},
		},
		{
			srm: subscribeDoneMessage{
				SusbcribeID:   17,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: false,
			},
			buf: []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{
				0x0a, 0x0b, 0x0c, 0x0d,
				0x0b,
				0x11,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x00,
			},
		},
		{
			srm: subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: true,
				FinalGroup:    0,
				FinalObject:   0,
			},
			buf:    []byte{},
			expect: []byte{0x0b, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00},
		},
		{
			srm: subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: true,
				FinalGroup:    2,
				FinalObject:   3,
			},
			buf: []byte{},
			expect: []byte{
				0x0b,
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x01,
				0x02,
				0x03,
			},
		},
		{
			srm: subscribeDoneMessage{
				SusbcribeID:   17,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: true,
				FinalGroup:    2,
				FinalObject:   3,
			},
			buf: []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{
				0x0a, 0x0b, 0x0c, 0x0d,
				0x0b,
				0x11,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x01,
				0x02,
				0x03,
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.srm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeDoneMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *subscribeDoneMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x00, 0x00, 0x00, 0x01, 0x00, 0x00,
			}),
			expect: &subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: true,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x01,
				0x02,
				0x03,
			}),
			expect: &subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: true,
				FinalGroup:    2,
				FinalObject:   3,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x00,
			}),
			expect: &subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    1,
				ReasonPhrase:  "reason",
				ContentExists: false,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x00, 0x00, 0x00, 0x00,
			}),
			expect: &subscribeDoneMessage{
				SusbcribeID:   0,
				StatusCode:    0,
				ReasonPhrase:  "",
				ContentExists: false,
				FinalGroup:    0,
				FinalObject:   0,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x00,
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x07,
			}),
			expect: nil,
			err:    errors.New("invalid use of ContentExists byte"),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseSubscribeDoneMessage()
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

func TestAnnounceMessageAppend(t *testing.T) {
	cases := []struct {
		am     announceMessage
		buf    []byte
		expect []byte
	}{
		{
			am: announceMessage{
				TrackNamespace:         "",
				TrackRequestParameters: parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(announceMessageType), 0x00, 0x00,
			},
		},
		{
			am: announceMessage{
				TrackNamespace:         "tracknamespace",
				TrackRequestParameters: parameters{},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(announceMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e', 0x00},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.am.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *announceMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewBuffer(
				append(append([]byte{0x09}, "trackname"...), 0x00),
			),
			expect: &announceMessage{
				TrackNamespace:         "trackname",
				TrackRequestParameters: parameters{},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseAnnounceMessage()
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

func TestAnnounceOkMessageAppend(t *testing.T) {
	cases := []struct {
		aom    announceOkMessage
		buf    []byte
		expect []byte
	}{
		{
			aom: announceOkMessage{
				TrackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(announceOkMessageType), 0x00,
			},
		},
		{
			aom: announceOkMessage{
				TrackNamespace: "tracknamespace",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(announceOkMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aom.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceOkMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *announceOkMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r: bytes.NewReader(append([]byte{0x0E}, "tracknamespace"...)),
			expect: &announceOkMessage{
				TrackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			r: bytes.NewReader(append([]byte{0x05}, "tracknamespace"...)),
			expect: &announceOkMessage{
				TrackNamespace: "track",
			},
			err: nil,
		},
		{
			r:      bytes.NewReader(append([]byte{0x0F}, "tracknamespace"...)),
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseAnnounceOkMessage()
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

func TestAnnounceErrorMessageAppend(t *testing.T) {
	cases := []struct {
		aem    announceErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			aem: announceErrorMessage{
				TrackNamespace: "",
				ErrorCode:      0,
				ReasonPhrase:   "",
			},
			buf: []byte{},
			expect: []byte{
				byte(announceErrorMessageType), 0x00, 0x00, 0x00,
			},
		},
		{
			aem: announceErrorMessage{
				TrackNamespace: "trackname",
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(announceErrorMessageType), 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
		{
			aem: announceErrorMessage{
				TrackNamespace: "trackname",
				ErrorCode:      1,
				ReasonPhrase:   "reason",
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, byte(announceErrorMessageType), 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aem.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceErrorMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *announceErrorMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x02, 0x03}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				append(append(append([]byte{0x0e}, "tracknamespace"...), 0x01, 0x0d), "reason phrase"...),
			),
			expect: &announceErrorMessage{
				TrackNamespace: "tracknamespace",
				ErrorCode:      1,
				ReasonPhrase:   "reason phrase",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseAnnounceErrorMessage()
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

func TestUnannounceMessageAppend(t *testing.T) {
	cases := []struct {
		uam    unannounceMessage
		buf    []byte
		expect []byte
	}{
		{
			uam: unannounceMessage{
				TrackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(unannounceMessageType), 0x00,
			},
		},
		{
			uam: unannounceMessage{
				TrackNamespace: "tracknamespace",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(unannounceMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.uam.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseUnannounceMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *unannounceMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r: bytes.NewReader(append([]byte{0x0E}, "tracknamespace"...)),
			expect: &unannounceMessage{
				TrackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			r: bytes.NewReader(append([]byte{0x05}, "tracknamespace"...)),
			expect: &unannounceMessage{
				TrackNamespace: "track",
			},
			err: nil,
		},
		{
			r:      bytes.NewReader(append([]byte{0x0F}, "tracknamespace"...)),
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseUnannounceMessage()
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

func TestGoAwayMessageAppend(t *testing.T) {
	cases := []struct {
		gam    goAwayMessage
		buf    []byte
		expect []byte
	}{
		{
			gam: goAwayMessage{
				NewSessionURI: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(goAwayMessageType), 0x00,
			},
		},
		{
			gam: goAwayMessage{
				NewSessionURI: "uri",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(goAwayMessageType), 0x03, 'u', 'r', 'i',
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.gam.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseGoAwayMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *goAwayMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r: bytes.NewReader(append([]byte{0x03}, "uri"...)),
			expect: &goAwayMessage{
				NewSessionURI: "uri",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseGoAwayMessage()
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

func TestAnnounceCancelMessageAppend(t *testing.T) {
	cases := []struct {
		acm    announceCancelMessage
		buf    []byte
		expect []byte
	}{
		{
			acm: announceCancelMessage{
				TrackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(announceCancelMessageType), 0x00,
			},
		},
		{
			acm: announceCancelMessage{
				TrackNamespace: "tracknamespace",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(announceCancelMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.acm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceCancelMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *announceCancelMessage
		err    error
	}{
		{
			r:      nil,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r: bytes.NewReader(append([]byte{0x0E}, "tracknamespace"...)),
			expect: &announceCancelMessage{
				TrackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			r: bytes.NewReader(append([]byte{0x05}, "tracknamespace"...)),
			expect: &announceCancelMessage{
				TrackNamespace: "track",
			},
			err: nil,
		},
		{
			r:      bytes.NewReader(append([]byte{0x0F}, "tracknamespace"...)),
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := loggingParser{
				logger: slog.Default(),
				reader: tc.r,
			}
			res, err := p.parseAnnounceCancelMessage()
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
