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
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageType), 0x04, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: objectMessage{
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(objectMessageType), 0x07, 0x01, 0x02, 0x03, 0x04,
				0x01, 0x02, 0x03,
			},
		},
		{
			om: objectMessage{
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{0x01, 0x02, 0x03},
			expect: []byte{
				0x01, 0x02, 0x03,
				byte(objectMessageType), 0x07, 0x01, 0x02, 0x03, 0x04,
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
		len    int
		expect *objectMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r:      bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			len:    9,
			expect: nil,
			err:    io.ErrUnexpectedEOF,
		},
		{
			r:   bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00}),
			len: 4,
			expect: &objectMessage{
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:   bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			len: 4,
			expect: &objectMessage{
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   []byte{},
			},
			err: nil,
		},
		{
			r:   bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x0a, 0x0b, 0x0c, 0x0d}),
			len: 8,
			expect: &objectMessage{
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
			res, err := parseObjectMessage(tc.r, tc.len)
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
				byte(setupMessageType), 0x01, 0x00,
			},
		},
		{
			csm: clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters:   parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x02, 0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
			},
		},
		{
			csm: clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters:   parameters{pathParameterKey: pathParameter("A")},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x05, 0x01, DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 'A',
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
		len    int
		expect *clientSetupMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
			}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader([]byte{
				0x01,
			}),
			len:    2,
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				0x02, DRAFT_IETF_MOQ_TRANSPORT_00, DRAFT_IETF_MOQ_TRANSPORT_00 + 1,
			}),
			len:    2,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader([]byte{
				0x02, DRAFT_IETF_MOQ_TRANSPORT_00, DRAFT_IETF_MOQ_TRANSPORT_00 + 1,
			}),
			len: 3,
			expect: &clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00, DRAFT_IETF_MOQ_TRANSPORT_00 + 1},
				SetupParameters:   parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
			}),
			len: 2,
			expect: &clientSetupMessage{
				SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters:   parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
			}),
			len:    3,
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseClientSetupMessage(tc.r, tc.len)
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
				byte(setupMessageType), 0x01, 0x00,
			},
		},
		{
			ssm: serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x01, 0x00,
			},
		},
		{
			ssm: serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: parameters{roleParameterKey: roleParameter(ingestionRole)},
			},
			buf: []byte{},
			expect: []byte{
				byte(setupMessageType), 0x04, 0x00, 0x00, 0x01, 0x01,
			},
		},
		{
			ssm: serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: parameters{pathParameterKey: pathParameter("A")},
			},
			buf: []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02,
				byte(setupMessageType), 0x04, 0x00, 0x01, 0x01, 'A',
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
		len    int
		expect *serverSetupMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01,
			}),
			len:    4,
			expect: nil,
			err:    io.EOF,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00,
			}),
			len: 1,
			expect: &serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 'A',
			}),
			len: 4,
			expect: &serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: parameters{pathParameterKey: pathParameter("A")},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c, 0x0d,
			}),
			len: 4,
			expect: &serverSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: parameters{pathParameterKey: pathParameter("A")},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseServerSetupMessage(tc.r, tc.len)
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
				FullTrackName:          "",
				TrackRequestParameters: parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeRequestMessageType), 0x01, 0x00,
			},
		},
		{
			srm: subscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: parameters{},
			},
			buf:    []byte{},
			expect: append([]byte{byte(subscribeRequestMessageType), 0x0a, 0x09}, "trackname"...),
		},
		{
			srm: subscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: parameters{pathParameterKey: pathParameter("A")},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeRequestMessageType), 0x0d, 0x09}, "trackname"...), []byte{0x01, 0x01, 'A'}...),
		},
		{
			srm: subscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: parameters{pathParameterKey: pathParameter("A")},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, byte(subscribeRequestMessageType), 0x0d, 0x09}, "trackname"...), []byte{0x01, 0x01, 'A'}...),
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
		len    int
		expect *subscribeRequestMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append([]byte{0x09}, "trackname"...),
			),
			len:    7,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append([]byte{0x09}, "trackname"...),
			),
			len: 10,
			expect: &subscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x01, 0x01),
			),
			len:    5,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x01, 'A'),
			),
			len: 13,
			expect: &subscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: parameters{pathParameterKey: pathParameter("A")},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			),
			len: 13,
			expect: &subscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: parameters{pathParameterKey: pathParameter("A")},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append([]byte{0x09}, "trackname"...),
			),
			len:    12,
			expect: nil,
			err:    io.EOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseSubscribeRequestMessage(tc.r, tc.len)
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
				FullTrackName: "",
				TrackID:       0,
				Expires:       0,
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeOkMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			som: subscribeOkMessage{
				FullTrackName: "fulltrackname",
				TrackID:       17,
				Expires:       1000,
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeOkMessageType), 0x11, 0x0d}, "fulltrackname"...), 0x11, 0x43, 0xe8),
		},
		{
			som: subscribeOkMessage{
				FullTrackName: "fulltrackname",
				TrackID:       17,
				Expires:       1000,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, byte(subscribeOkMessageType), 0x11, 0x0d}, "fulltrackname"...), 0x11, 0x43, 0xe8),
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
		len    int
		expect *subscribeOkMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x00),
			),
			len:    50,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x00),
			),
			len:    5,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x10),
			),
			len: 12,
			expect: &subscribeOkMessage{
				FullTrackName: "trackname",
				TrackID:       1,
				Expires:       0x10 * time.Millisecond,
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseSubscribeOkMessage(tc.r, tc.len)
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
				FullTrackName: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(subscribeErrorMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				FullTrackName: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeErrorMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				FullTrackName: "fulltrackname",
				ErrorCode:     12,
				ReasonPhrase:  "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeErrorMessageType), 0x16, 0x0d}, "fulltrackname"...), []byte{0x0c, 0x06, 'r', 'e', 'a', 's', 'o', 'n'}...),
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
		len    int
		expect *subscribeErrorMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x02, 0x03, 0x04}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), append([]byte{0x01, 0x0c}, "error phrase"...)...),
			),
			len:    100,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), append([]byte{0x01, 0x0c}, "error phrase"...)...),
			),
			len:    10,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), append([]byte{0x01, 0x0c}, "error phrase"...)...),
			),
			len: 24,
			expect: &subscribeErrorMessage{
				FullTrackName: "trackname",
				ErrorCode:     1,
				ReasonPhrase:  "error phrase",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseSubscribeErrorMessage(tc.r, tc.len)
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
				TrackNamespace:  "",
				TrackParameters: parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(announceMessageType), 0x01, 0x00,
			},
		},
		{
			am: announceMessage{
				TrackNamespace:  "tracknamespace",
				TrackParameters: parameters{},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(announceMessageType), 0x0f, 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
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
		len    int
		expect *announceMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewBuffer(
				append([]byte{0x09}, "trackname"...),
			),
			len: 10,
			expect: &announceMessage{
				TrackNamespace:  "trackname",
				TrackParameters: parameters{},
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseAnnounceMessage(tc.r, tc.len)
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
		len    int
		expect *announceOkMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:   bytes.NewReader([]byte("tracknamespace")),
			len: 0,
			expect: &announceOkMessage{
				TrackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			r:   bytes.NewReader([]byte("tracknamespace")),
			len: 5,
			expect: &announceOkMessage{
				TrackNamespace: "track",
			},
			err: nil,
		},
		{
			r:      bytes.NewReader([]byte("tracknamespace")),
			len:    20,
			expect: nil,
			err:    io.ErrUnexpectedEOF,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseAnnounceOkMessage(tc.r, tc.len)
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
		sem    subscribeErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			sem: subscribeErrorMessage{
				FullTrackName: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
			buf: []byte{},
			expect: []byte{
				byte(subscribeErrorMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: subscribeErrorMessage{
				FullTrackName: "trackname",
				ErrorCode:     1,
				ReasonPhrase:  "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(subscribeErrorMessageType), 0x12, 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
		{
			sem: subscribeErrorMessage{
				FullTrackName: "trackname",
				ErrorCode:     1,
				ReasonPhrase:  "reason",
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, byte(subscribeErrorMessageType), 0x12, 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sem.append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceErrorMessage(t *testing.T) {
	cases := []struct {
		r      messageReader
		len    int
		expect *announceErrorMessage
		err    error
	}{
		{
			r:      nil,
			len:    0,
			expect: nil,
			err:    errInvalidMessageReader,
		},
		{
			r:      bytes.NewReader([]byte{0x01, 0x02, 0x03}),
			len:    0,
			expect: nil,
			err:    errInvalidMessageEncoding,
		},
		{
			r: bytes.NewReader(
				append(append(append([]byte{0x0e}, "tracknamespace"...), 0x01, 0x0d), "reason phrase"...),
			),
			len: 30,
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
			res, err := parseAnnounceErrorMessage(tc.r, tc.len)
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
			gam: goAwayMessage{},
			buf: []byte{},
			expect: []byte{
				byte(goAwayMessageType), 0x00,
			},
		},
		{
			gam: goAwayMessage{},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(goAwayMessageType), 0x00,
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
		expect *goAwayMessage
		len    int
		err    error
	}{
		{
			expect: &goAwayMessage{},
			len:    0,
			err:    nil,
		},
		{
			expect: nil,
			len:    10,
			err:    errInvalidMessageEncoding,
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res, err := parseGoAwayMessage(tc.len)
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
