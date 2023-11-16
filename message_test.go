package moqtransport

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestObjectMessageAppend(t *testing.T) {
	cases := []struct {
		om     objectMessage
		buf    []byte
		expect []byte
	}{
		{
			om: objectMessage{
				HasLength:       true,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageLenType), 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: objectMessage{
				HasLength:       true,
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageLenType), 0x01, 0x02, 0x03, 0x04,
				0x03, 0x01, 0x02, 0x03,
			},
		},
		{
			om: objectMessage{
				HasLength:       true,
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{0x01, 0x02, 0x03},
			expect: []byte{
				0x01, 0x02, 0x03,
				byte(objectMessageLenType), 0x01, 0x02, 0x03, 0x04,
				0x03, 0x01, 0x02, 0x03,
			},
		},
		{
			om: objectMessage{
				HasLength:       false,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageNoLenType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: objectMessage{
				HasLength:       false,
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageNoLenType), 0x01, 0x02, 0x03, 0x04,
				0x01, 0x02, 0x03,
			},
		},
		{
			om: objectMessage{
				HasLength:       false,
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{0x01, 0x02, 0x03},
			expect: []byte{
				0x01, 0x02, 0x03,
				byte(objectMessageNoLenType), 0x01, 0x02, 0x03, 0x04,
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
		msgTyp uint64
		expect *objectMessage
		err    error
	}{
		{
			r:      bytes.NewReader([]byte{}),
			msgTyp: 17,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
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
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00}),
			msgTyp: 0,
			expect: &objectMessage{
				HasLength:       true,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 0,
			expect: nil,
			err:    io.ErrUnexpectedEOF,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00}),
			msgTyp: 0,
			expect: &objectMessage{
				HasLength:       true,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 0,
			expect: &objectMessage{
				HasLength:       true,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x04, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 0,
			expect: &objectMessage{
				HasLength:       true,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}),
			msgTyp: 2,
			expect: &objectMessage{
				HasLength:       false,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 2,
			expect: &objectMessage{
				HasLength:       false,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}),
			msgTyp: 2,
			expect: &objectMessage{
				HasLength:       false,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 2,
			expect: &objectMessage{
				HasLength:       false,
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := parser{
				logger: &log.Logger{},
				reader: tc.r,
			}
			res, err := p.parseObjectMessage(tc.msgTyp)
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
			p := parser{
				logger: &log.Logger{},
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
					v: ingestionRole,
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
			p := parser{
				logger: &log.Logger{},
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
		loc    location
		buf    []byte
		expect []byte
	}{
		{
			loc:    location{},
			buf:    []byte{},
			expect: []byte{0x00},
		},
		{
			loc: location{
				mode:  1,
				value: 0,
			},
			buf:    []byte{},
			expect: []byte{0x01, 0x00},
		},
		{
			loc: location{
				mode:  2,
				value: 10,
			},
			buf:    []byte{},
			expect: []byte{0x02, 0x0A},
		},
		{
			loc: location{
				mode:  3,
				value: 8,
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
		expect location
		err    error
	}{
		{
			r:      nil,
			expect: location{},
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			expect: location{},
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{0x00}),
			expect: location{
				mode:  0,
				value: 0,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{0x01, 0x02}),
			expect: location{
				mode:  1,
				value: 2,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := parser{
				logger: &log.Logger{},
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
		srm    subscribeRequestMessage
		buf    []byte
		expect []byte
	}{
		{
			srm: subscribeRequestMessage{
				TrackNamespace: "",
				TrackName:      "",
				StartGroup:     location{},
				StartObject:    location{},
				EndGroup:       location{},
				EndObject:      location{},
				Parameters:     parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeRequestMessageType), 0x00, 0x00, 0x00, 0x00, 0x00, 0x0, 0x00,
			},
		},
		{
			srm: subscribeRequestMessage{
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     location{mode: 0, value: 0},
				StartObject:    location{mode: 1, value: 0},
				EndGroup:       location{mode: 2, value: 0},
				EndObject:      location{mode: 3, value: 0},
				Parameters:     parameters{},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeRequestMessageType), 0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x00),
		},
		{
			srm: subscribeRequestMessage{
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     location{},
				StartObject:    location{},
				EndGroup:       location{},
				EndObject:      location{},
				Parameters:     parameters{pathParameterKey: stringParameter{k: pathParameterKey, v: "A"}},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeRequestMessageType), 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A'}...),
		},
		{
			srm: subscribeRequestMessage{
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     location{},
				StartObject:    location{},
				EndGroup:       location{},
				EndObject:      location{},
				Parameters:     parameters{pathParameterKey: stringParameter{k: pathParameterKey, v: "A"}},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, byte(subscribeRequestMessageType), 0x02, 'n', 's', 0x09}, "trackname"...), []byte{0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A'}...),
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
		expect *subscribeRequestMessage
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
				append(append([]byte{0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x00, 0x00, 0x00, 0x00),
			),
			expect: &subscribeRequestMessage{
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     location{},
				StartObject:    location{},
				EndGroup:       location{},
				EndObject:      location{},
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
				append(append([]byte{0x02, 'n', 's', 0x09}, "trackname"...), 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x01, 0x01, 'A'),
			),
			expect: &subscribeRequestMessage{
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     location{mode: 1, value: 2},
				StartObject:    location{mode: 1, value: 2},
				EndGroup:       location{mode: 1, value: 2},
				EndObject:      location{mode: 1, value: 2},
				Parameters:     parameters{pathParameterKey: stringParameter{k: pathParameterKey, v: "A"}},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x02, 'n', 's', 0x09}, "trackname"...), 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			),
			expect: &subscribeRequestMessage{
				TrackNamespace: "ns",
				TrackName:      "trackname",
				StartGroup:     location{},
				StartObject:    location{},
				EndGroup:       location{},
				EndObject:      location{},
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
			p := parser{
				logger: &log.Logger{},
				reader: tc.r,
			}
			res, err := p.parseSubscribeRequestMessage()
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
				TrackNamespace: "",
				TrackName:      "",
				TrackID:        0,
				Expires:        0,
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeOkMessageType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			som: subscribeOkMessage{
				TrackNamespace: "",
				TrackName:      "fulltrackname",
				TrackID:        17,
				Expires:        1000,
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeOkMessageType), 0x00, 0x0d}, "fulltrackname"...), 0x11, 0x43, 0xe8),
		},
		{
			som: subscribeOkMessage{
				TrackName: "fulltrackname",
				TrackID:   17,
				Expires:   1000,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, byte(subscribeOkMessageType), 0x00, 0x0d}, "fulltrackname"...), 0x11, 0x43, 0xe8),
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
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x00),
			),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x00),
			),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x00, 0x09}, "trackname"...), 0x01, 0x10),
			),
			expect: &subscribeOkMessage{
				TrackNamespace: "",
				TrackName:      "trackname",
				TrackID:        1,
				Expires:        0x10 * time.Millisecond,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := parser{
				logger: &log.Logger{},
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
				TrackNamespace: "",
				TrackName:      "",
				ErrorCode:      0,
				ReasonPhrase:   "",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(subscribeErrorMessageType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				TrackNamespace: "",
				TrackName:      "",
				ErrorCode:      0,
				ReasonPhrase:   "",
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeErrorMessageType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				TrackNamespace: "",
				TrackName:      "fulltrackname",
				ErrorCode:      12,
				ReasonPhrase:   "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeErrorMessageType), 0x00, 0x0d}, "fulltrackname"...), []byte{0x0c, 0x06, 'r', 'e', 'a', 's', 'o', 'n'}...),
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
				append(append([]byte{0x00, 0x09}, "trackname"...), append([]byte{0x01, 0x0c}, "error phrase"...)...),
			),
			expect: &subscribeErrorMessage{
				TrackName:    "trackname",
				ErrorCode:    1,
				ReasonPhrase: "error phrase",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := parser{
				logger: &log.Logger{},
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
				TrackNamespace: "",
				TrackName:      "",
			},
			buf: []byte{},
			expect: []byte{
				byte(unsubscribeMessageType), 0x00, 0x00,
			},
		},
		{
			usm: unsubscribeMessage{
				TrackNamespace: "tracknamespace",
				TrackName:      "",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(unsubscribeMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e', 0x00},
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
			r: bytes.NewReader(append([]byte{0, 9}, "trackname"...)),
			expect: &unsubscribeMessage{
				TrackNamespace: "",
				TrackName:      "trackname",
			},
			err: nil,
		},
		{
			r:      bytes.NewReader(append([]byte{0xA}, "track"...)),
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := parser{
				logger: &log.Logger{},
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

func TestSubscribeFinMessageAppend(t *testing.T) {
	cases := []struct {
		sfm    subscribeFinMessage
		buf    []byte
		expect []byte
	}{
		{
			sfm: subscribeFinMessage{
				TrackNamespace: "",
				TrackName:      "",
				FinalGroup:     0,
				FinalObject:    0,
			},
			buf:    []byte{},
			expect: []byte{0x0b, 0x00, 0x00, 0x00, 0x00},
		},
		{
			sfm: subscribeFinMessage{
				TrackNamespace: "Test",
				TrackName:      "Name",
				FinalGroup:     1,
				FinalObject:    2,
			},
			buf:    []byte{},
			expect: []byte{0x0b, 0x04, 'T', 'e', 's', 't', 0x04, 'N', 'a', 'm', 'e', 0x01, 0x02},
		},
		{
			sfm: subscribeFinMessage{
				TrackNamespace: "Test",
				TrackName:      "Name",
				FinalGroup:     1,
				FinalObject:    2,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{0x0a, 0x0b, 0x0c, 0x0d, 0x0b, 0x04, 'T', 'e', 's', 't', 0x04, 'N', 'a', 'm', 'e', 0x01, 0x02},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sfm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeFinMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *subscribeFinMessage
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
				0x00, 0x00, 0x00, 0x00,
			}),
			expect: &subscribeFinMessage{
				TrackNamespace: "",
				TrackName:      "",
				FinalGroup:     0,
				FinalObject:    0,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x05, 's', 'p', 'a', 'c', 'e',
				0x04, 'n', 'a', 'm', 'e',
				0x01,
				0x02,
			}),
			expect: &subscribeFinMessage{
				TrackNamespace: "space",
				TrackName:      "name",
				FinalGroup:     1,
				FinalObject:    2,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := parser{
				logger: &log.Logger{},
				reader: tc.r,
			}
			res, err := p.parseSubscribeFinMessage()
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

func TestSubscribeRstMessageAppend(t *testing.T) {
	cases := []struct {
		srm    subscribeRstMessage
		buf    []byte
		expect []byte
	}{
		{
			srm: subscribeRstMessage{
				TrackNamespace: "",
				TrackName:      "",
				ErrorCode:      0,
				ReasonPhrase:   "",
				FinalGroup:     0,
				FinalObject:    0,
			},
			buf:    []byte{},
			expect: []byte{0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			srm: subscribeRstMessage{
				TrackNamespace: "space",
				TrackName:      "name",
				ErrorCode:      1,
				ReasonPhrase:   "reason",
				FinalGroup:     2,
				FinalObject:    3,
			},
			buf: []byte{},
			expect: []byte{
				0x0c,
				0x05, 's', 'p', 'a', 'c', 'e',
				0x04, 'n', 'a', 'm', 'e',
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x02, 0x03,
			},
		},
		{
			srm: subscribeRstMessage{
				TrackNamespace: "space",
				TrackName:      "name",
				ErrorCode:      1,
				ReasonPhrase:   "reason",
				FinalGroup:     2,
				FinalObject:    3,
			},
			buf: []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: []byte{
				0x0a, 0x0b, 0x0c, 0x0d,
				0x0c,
				0x05, 's', 'p', 'a', 'c', 'e',
				0x04, 'n', 'a', 'm', 'e',
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x02, 0x03,
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

func TestParseSubscribeRstMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		expect *subscribeRstMessage
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
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			}),
			expect: &subscribeRstMessage{
				TrackNamespace: "",
				TrackName:      "",
				ErrorCode:      0,
				ReasonPhrase:   "",
				FinalGroup:     0,
				FinalObject:    0,
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x05, 's', 'p', 'a', 'c', 'e',
				0x04, 'n', 'a', 'm', 'e',
				0x01,
				0x06, 'r', 'e', 'a', 's', 'o', 'n',
				0x02,
				0x03,
			}),
			expect: &subscribeRstMessage{
				TrackNamespace: "space",
				TrackName:      "name",
				ErrorCode:      1,
				ReasonPhrase:   "reason",
				FinalGroup:     2,
				FinalObject:    3,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			p := parser{
				logger: &log.Logger{},
				reader: tc.r,
			}
			res, err := p.parseSubscribeRstMessage()
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
			p := parser{
				logger: &log.Logger{},
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
			p := parser{
				logger: &log.Logger{},
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
			p := parser{
				logger: &log.Logger{},
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
			p := parser{
				logger: &log.Logger{},
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
			p := parser{
				logger: &log.Logger{},
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
