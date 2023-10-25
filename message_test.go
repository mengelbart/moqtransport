package moqtransport

import (
	"bytes"
	"fmt"
	"io"
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
				hasLength:       true,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageLenType), 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: objectMessage{
				hasLength:       true,
				trackID:         1,
				groupSequence:   2,
				objectSequence:  3,
				objectSendOrder: 4,
				objectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageLenType), 0x01, 0x02, 0x03, 0x04,
				0x03, 0x01, 0x02, 0x03,
			},
		},
		{
			om: objectMessage{
				hasLength:       true,
				trackID:         1,
				groupSequence:   2,
				objectSequence:  3,
				objectSendOrder: 4,
				objectPayload:   []byte{0x01, 0x02, 0x03},
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
				hasLength:       false,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageNoLenType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: objectMessage{
				hasLength:       false,
				trackID:         1,
				groupSequence:   2,
				objectSequence:  3,
				objectSendOrder: 4,
				objectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageNoLenType), 0x01, 0x02, 0x03, 0x04,
				0x01, 0x02, 0x03,
			},
		},
		{
			om: objectMessage{
				hasLength:       false,
				trackID:         1,
				groupSequence:   2,
				objectSequence:  3,
				objectSendOrder: 4,
				objectPayload:   []byte{0x01, 0x02, 0x03},
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
				hasLength:       true,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{},
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
				hasLength:       true,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 0,
			expect: &objectMessage{
				hasLength:       true,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x04, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 0,
			expect: &objectMessage{
				hasLength:       true,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}),
			msgTyp: 2,
			expect: &objectMessage{
				hasLength:       false,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 2,
			expect: &objectMessage{
				hasLength:       false,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}),
			msgTyp: 2,
			expect: &objectMessage{
				hasLength:       false,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			msgTyp: 2,
			expect: &objectMessage{
				hasLength:       false,
				trackID:         0,
				groupSequence:   0,
				objectSequence:  0,
				objectSendOrder: 0,
				objectPayload:   []byte{0x0a, 0x0b, 0x0c, 0x0d},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseObjectMessage(tc.r, tc.msgTyp)
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
				supportedVersions: nil,
				setupParameters:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x00, 0x00,
			},
		},
		{
			csm: clientSetupMessage{
				supportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				setupParameters:   parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x01, DRAFT_IETF_MOQ_TRANSPORT_00, 0x00,
			},
		},
		{
			csm: clientSetupMessage{
				supportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				setupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x01, DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 0x01, 'A',
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
				0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
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
				0x02, DRAFT_IETF_MOQ_TRANSPORT_00, DRAFT_IETF_MOQ_TRANSPORT_00 + 1,
			}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x02, DRAFT_IETF_MOQ_TRANSPORT_00, DRAFT_IETF_MOQ_TRANSPORT_00 + 1, 0x00,
			}),
			expect: &clientSetupMessage{
				supportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00, DRAFT_IETF_MOQ_TRANSPORT_00 + 1},
				setupParameters:   parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, DRAFT_IETF_MOQ_TRANSPORT_00, 0x00,
			}),
			expect: &clientSetupMessage{
				supportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				setupParameters:   parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
			}),
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseClientSetupMessage(tc.r)
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
				selectedVersion: 0,
				setupParameters: nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x00, 0x00,
			},
		},
		{
			ssm: serverSetupMessage{
				selectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				setupParameters: parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x00, 0x00,
			},
		},
		{
			ssm: serverSetupMessage{
				selectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				setupParameters: parameters{roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: ingestionRole,
				}},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x00, 0x01, 0x00, 0x01, 0x01,
			},
		},
		{
			ssm: serverSetupMessage{
				selectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				setupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			buf: []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02,
				byte(setupMessageType), 0x00, 0x01, 0x01, 0x01, 'A',
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
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01,
			}),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x00,
			}),
			expect: &serverSetupMessage{
				selectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				setupParameters: parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 0x01, 'A',
			}),
			expect: &serverSetupMessage{
				selectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				setupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c, 0x0d,
			}),
			expect: &serverSetupMessage{
				selectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				setupParameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseServerSetupMessage(tc.r)
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

func TestSubscribeMessageAppend(t *testing.T) {
	cases := []struct {
		srm    subscribeMessage
		buf    []byte
		expect []byte
	}{
		{
			srm: subscribeMessage{
				trackNamespace: "",
				trackName:      "",
				parameters:     parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeMessageType), 0x00, 0x00, 0x00,
			},
		},
		{
			srm: subscribeMessage{
				trackNamespace: "",
				trackName:      "trackname",
				parameters:     parameters{},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeMessageType), 0x00, 0x09}, "trackname"...), 0x00),
		},
		{
			srm: subscribeMessage{
				trackNamespace: "",
				trackName:      "trackname",
				parameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeMessageType), 0x00, 0x09}, "trackname"...), []byte{0x01, 0x01, 0x01, 'A'}...),
		},
		{
			srm: subscribeMessage{
				trackNamespace: "",
				trackName:      "trackname",
				parameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, byte(subscribeMessageType), 0x00, 0x09}, "trackname"...), []byte{0x01, 0x01, 0x01, 'A'}...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.srm.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeMessage(t *testing.T) {
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
				append(append([]byte{0x00, 0x09}, "trackname"...), 0x00),
			),
			expect: &subscribeMessage{
				trackNamespace: "",
				trackName:      "trackname",
				parameters:     parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x00, 0x09}, "trackname"...), 0x01, 0x01, 0x01),
			),
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x00, 0x09}, "trackname"...), 0x01, 0x01, 0x01, 'A'),
			),
			expect: &subscribeMessage{
				trackNamespace: "",
				trackName:      "trackname",
				parameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x00, 0x09}, "trackname"...), 0x01, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			),
			expect: &subscribeMessage{
				trackNamespace: "",
				trackName:      "trackname",
				parameters: parameters{pathParameterKey: stringParameter{
					k: pathParameterKey,
					v: "A",
				}},
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
			res, err := parseSubscribeMessage(tc.r)
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
				trackNamespace: "",
				trackName:      "",
				trackID:        0,
				expires:        0,
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeOkMessageType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			som: subscribeOkMessage{
				trackNamespace: "",
				trackName:      "fulltrackname",
				trackID:        17,
				expires:        1000,
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeOkMessageType), 0x00, 0x0d}, "fulltrackname"...), 0x11, 0x43, 0xe8),
		},
		{
			som: subscribeOkMessage{
				trackName: "fulltrackname",
				trackID:   17,
				expires:   1000,
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
				trackNamespace: "",
				trackName:      "trackname",
				trackID:        1,
				expires:        0x10 * time.Millisecond,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseSubscribeOkMessage(tc.r)
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
				trackNamespace: "",
				trackName:      "",
				errorCode:      0,
				reasonPhrase:   "",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(subscribeErrorMessageType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				trackNamespace: "",
				trackName:      "",
				errorCode:      0,
				reasonPhrase:   "",
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeErrorMessageType), 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				trackNamespace: "",
				trackName:      "fulltrackname",
				errorCode:      12,
				reasonPhrase:   "reason",
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
				trackName:    "trackname",
				errorCode:    1,
				reasonPhrase: "error phrase",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseSubscribeErrorMessage(tc.r)
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
				trackNamespace: "",
				trackName:      "",
			},
			buf: []byte{},
			expect: []byte{
				byte(unsubscribeMessageType), 0x00, 0x00,
			},
		},
		{
			usm: unsubscribeMessage{
				trackNamespace: "tracknamespace",
				trackName:      "",
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
				trackNamespace: "",
				trackName:      "trackname",
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
			res, err := parseUnsubscribeMessage(tc.r)
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
				trackNamespace: "",
				trackName:      "",
				finalGroup:     0,
				finalObject:    0,
			},
			buf:    []byte{},
			expect: []byte{0x0b, 0x00, 0x00, 0x00, 0x00},
		},
		{
			sfm: subscribeFinMessage{
				trackNamespace: "Test",
				trackName:      "Name",
				finalGroup:     1,
				finalObject:    2,
			},
			buf:    []byte{},
			expect: []byte{0x0b, 0x04, 'T', 'e', 's', 't', 0x04, 'N', 'a', 'm', 'e', 0x01, 0x02},
		},
		{
			sfm: subscribeFinMessage{
				trackNamespace: "Test",
				trackName:      "Name",
				finalGroup:     1,
				finalObject:    2,
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
				trackNamespace: "",
				trackName:      "",
				finalGroup:     0,
				finalObject:    0,
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
				trackNamespace: "space",
				trackName:      "name",
				finalGroup:     1,
				finalObject:    2,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseSubscribeFinMessage(tc.r)
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
				trackNamespace: "",
				trackName:      "",
				errorCode:      0,
				reasonPhrase:   "",
				finalGroup:     0,
				finalObject:    0,
			},
			buf:    []byte{},
			expect: []byte{0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			srm: subscribeRstMessage{
				trackNamespace: "space",
				trackName:      "name",
				errorCode:      1,
				reasonPhrase:   "reason",
				finalGroup:     2,
				finalObject:    3,
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
				trackNamespace: "space",
				trackName:      "name",
				errorCode:      1,
				reasonPhrase:   "reason",
				finalGroup:     2,
				finalObject:    3,
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
				trackNamespace: "",
				trackName:      "",
				errorCode:      0,
				reasonPhrase:   "",
				finalGroup:     0,
				finalObject:    0,
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
				trackNamespace: "space",
				trackName:      "name",
				errorCode:      1,
				reasonPhrase:   "reason",
				finalGroup:     2,
				finalObject:    3,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseSubscribeRstMessage(tc.r)
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
				trackNamespace:         "",
				trackRequestParameters: parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(announceMessageType), 0x00, 0x00,
			},
		},
		{
			am: announceMessage{
				trackNamespace:         "tracknamespace",
				trackRequestParameters: parameters{},
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
				trackNamespace:         "trackname",
				trackRequestParameters: parameters{},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseAnnounceMessage(tc.r)
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
				trackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(announceOkMessageType), 0x00,
			},
		},
		{
			aom: announceOkMessage{
				trackNamespace: "tracknamespace",
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
				trackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			r: bytes.NewReader(append([]byte{0x05}, "tracknamespace"...)),
			expect: &announceOkMessage{
				trackNamespace: "track",
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
			res, err := parseAnnounceOkMessage(tc.r)
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
				trackNamespace: "",
				errorCode:      0,
				reasonPhrase:   "",
			},
			buf: []byte{},
			expect: []byte{
				byte(announceErrorMessageType), 0x00, 0x00, 0x00,
			},
		},
		{
			aem: announceErrorMessage{
				trackNamespace: "trackname",
				errorCode:      1,
				reasonPhrase:   "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(announceErrorMessageType), 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
		{
			aem: announceErrorMessage{
				trackNamespace: "trackname",
				errorCode:      1,
				reasonPhrase:   "reason",
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
				trackNamespace: "tracknamespace",
				errorCode:      1,
				reasonPhrase:   "reason phrase",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseAnnounceErrorMessage(tc.r)
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
				trackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(unannounceMessageType), 0x00,
			},
		},
		{
			uam: unannounceMessage{
				trackNamespace: "tracknamespace",
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
				trackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			r: bytes.NewReader(append([]byte{0x05}, "tracknamespace"...)),
			expect: &unannounceMessage{
				trackNamespace: "track",
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
			res, err := parseUnannounceMessage(tc.r)
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
				newSessionURI: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(goAwayMessageType), 0x00,
			},
		},
		{
			gam: goAwayMessage{
				newSessionURI: "uri",
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
				newSessionURI: "uri",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseGoAwayMessage(tc.r)
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
