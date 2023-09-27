package transport

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
		om     ObjectMessage
		buf    []byte
		expect []byte
	}{
		{
			om: ObjectMessage{
				TrackID:         0,
				GroupSequence:   0,
				ObjectSequence:  0,
				ObjectSendOrder: 0,
				ObjectPayload:   nil,
			},
			buf: []byte{},
			expect: []byte{
				byte(ObjectMessageType), 0x04, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			om: ObjectMessage{
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{},
			expect: []byte{
				byte(ObjectMessageType), 0x07, 0x01, 0x02, 0x03, 0x04,
				0x01, 0x02, 0x03,
			},
		},
		{
			om: ObjectMessage{
				TrackID:         1,
				GroupSequence:   2,
				ObjectSequence:  3,
				ObjectSendOrder: 4,
				ObjectPayload:   []byte{0x01, 0x02, 0x03},
			},
			buf: []byte{0x01, 0x02, 0x03},
			expect: []byte{
				0x01, 0x02, 0x03,
				byte(ObjectMessageType), 0x07, 0x01, 0x02, 0x03, 0x04,
				0x01, 0x02, 0x03,
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.om.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseObjectMessage(t *testing.T) {
	cases := []struct {
		r      MessageReader
		len    int
		expect *ObjectMessage
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
			expect: &ObjectMessage{
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
			expect: &ObjectMessage{
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
			expect: &ObjectMessage{
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
				byte(SetupMessageType), 0x01, 0x00,
			},
		},
		{
			csm: ClientSetupMessage{
				SupportedVersions: []Version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters:   Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(SetupMessageType), 0x02, 0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
			},
		},
		{
			csm: ClientSetupMessage{
				SupportedVersions: []Version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters:   Parameters{pathParameterKey: pathParameter("A")},
			},
			buf: []byte{},
			expect: []byte{
				byte(SetupMessageType), 0x05, 0x01, DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 'A',
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
		r      MessageReader
		len    int
		expect *ClientSetupMessage
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
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{DRAFT_IETF_MOQ_TRANSPORT_00, DRAFT_IETF_MOQ_TRANSPORT_00 + 1},
				SetupParameters:   Parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				0x01, DRAFT_IETF_MOQ_TRANSPORT_00,
			}),
			len: 2,
			expect: &ClientSetupMessage{
				SupportedVersions: []Version{DRAFT_IETF_MOQ_TRANSPORT_00},
				SetupParameters:   Parameters{},
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
				byte(SetupMessageType), 0x01, 0x00,
			},
		},
		{
			ssm: ServerSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(SetupMessageType), 0x01, 0x00,
			},
		},
		{
			ssm: ServerSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: Parameters{roleParameterKey: roleParameter(ingestionRole)},
			},
			buf: []byte{},
			expect: []byte{
				byte(SetupMessageType), 0x04, 0x00, 0x00, 0x01, 0x01,
			},
		},
		{
			ssm: ServerSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: Parameters{pathParameterKey: pathParameter("A")},
			},
			buf: []byte{0x01, 0x02},
			expect: []byte{0x01, 0x02,
				byte(SetupMessageType), 0x04, 0x00, 0x01, 0x01, 'A',
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
		r      MessageReader
		len    int
		expect *ServerSetupMessage
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
			expect: &ServerSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: Parameters{},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 'A',
			}),
			len: 4,
			expect: &ServerSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: Parameters{pathParameterKey: pathParameter("A")},
			},
			err: nil,
		},
		{
			r: bytes.NewReader([]byte{
				DRAFT_IETF_MOQ_TRANSPORT_00, 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c, 0x0d,
			}),
			len: 4,
			expect: &ServerSetupMessage{
				SelectedVersion: DRAFT_IETF_MOQ_TRANSPORT_00,
				SetupParameters: Parameters{pathParameterKey: pathParameter("A")},
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
		srm    SubscribeRequestMessage
		buf    []byte
		expect []byte
	}{
		{
			srm: SubscribeRequestMessage{
				FullTrackName:          "",
				TrackRequestParameters: Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(SubscribeRequestMessageType), 0x01, 0x00,
			},
		},
		{
			srm: SubscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: Parameters{},
			},
			buf:    []byte{},
			expect: append([]byte{byte(SubscribeRequestMessageType), 0x0a, 0x09}, "trackname"...),
		},
		{
			srm: SubscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: Parameters{pathParameterKey: pathParameter("A")},
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(SubscribeRequestMessageType), 0x0d, 0x09}, "trackname"...), []byte{0x01, 0x01, 'A'}...),
		},
		{
			srm: SubscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: Parameters{pathParameterKey: pathParameter("A")},
			},
			buf:    []byte{0x01, 0x02, 0x03, 0x04},
			expect: append(append([]byte{0x01, 0x02, 0x03, 0x04, byte(SubscribeRequestMessageType), 0x0d, 0x09}, "trackname"...), []byte{0x01, 0x01, 'A'}...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.srm.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeRequestMessage(t *testing.T) {
	cases := []struct {
		r      MessageReader
		len    int
		expect *SubscribeRequestMessage
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
			expect: &SubscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: Parameters{},
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
			expect: &SubscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: Parameters{pathParameterKey: pathParameter("A")},
			},
			err: nil,
		},
		{
			r: bytes.NewReader(
				append(append([]byte{0x09}, "trackname"...), 0x01, 0x01, 'A', 0x0a, 0x0b, 0x0c),
			),
			len: 13,
			expect: &SubscribeRequestMessage{
				FullTrackName:          "trackname",
				TrackRequestParameters: Parameters{pathParameterKey: pathParameter("A")},
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
		som    SubscribeOkMessage
		buf    []byte
		expect []byte
	}{
		{
			som: SubscribeOkMessage{
				FullTrackName: "",
				TrackID:       0,
				Expires:       0,
			},
			buf: []byte{},
			expect: []byte{
				byte(SubscribeOkMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			som: SubscribeOkMessage{
				FullTrackName: "fulltrackname",
				TrackID:       17,
				Expires:       1000,
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(SubscribeOkMessageType), 0x11, 0x0d}, "fulltrackname"...), 0x11, 0x43, 0xe8),
		},
		{
			som: SubscribeOkMessage{
				FullTrackName: "fulltrackname",
				TrackID:       17,
				Expires:       1000,
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, byte(SubscribeOkMessageType), 0x11, 0x0d}, "fulltrackname"...), 0x11, 0x43, 0xe8),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.som.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeOkMessage(t *testing.T) {
	cases := []struct {
		r      MessageReader
		len    int
		expect *SubscribeOkMessage
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
			expect: &SubscribeOkMessage{
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
		sem    SubscribeErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			sem: SubscribeErrorMessage{
				FullTrackName: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(SubscribeErrorMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: SubscribeErrorMessage{
				FullTrackName: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
			buf: []byte{},
			expect: []byte{
				byte(SubscribeErrorMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: SubscribeErrorMessage{
				FullTrackName: "fulltrackname",
				ErrorCode:     12,
				ReasonPhrase:  "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(SubscribeErrorMessageType), 0x16, 0x0d}, "fulltrackname"...), []byte{0x0c, 0x06, 'r', 'e', 'a', 's', 'o', 'n'}...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sem.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseSubscribeErrorMessage(t *testing.T) {
	cases := []struct {
		r      MessageReader
		len    int
		expect *SubscribeErrorMessage
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
			expect: &SubscribeErrorMessage{
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
		am     AnnounceMessage
		buf    []byte
		expect []byte
	}{
		{
			am: AnnounceMessage{
				TrackNamespace:  "",
				TrackParameters: Parameters{},
			},
			buf: []byte{},
			expect: []byte{
				byte(AnnounceMessageType), 0x01, 0x00,
			},
		},
		{
			am: AnnounceMessage{
				TrackNamespace:  "tracknamespace",
				TrackParameters: Parameters{},
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(AnnounceMessageType), 0x0f, 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.am.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceMessage(t *testing.T) {
	cases := []struct {
		r      MessageReader
		len    int
		expect *AnnounceMessage
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
			expect: &AnnounceMessage{
				TrackNamespace:  "trackname",
				TrackParameters: Parameters{},
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
		aom    AnnounceOkMessage
		buf    []byte
		expect []byte
	}{
		{
			aom: AnnounceOkMessage{
				TrackNamespace: "",
			},
			buf: []byte{},
			expect: []byte{
				byte(AnnounceOkMessageType), 0x00,
			},
		},
		{
			aom: AnnounceOkMessage{
				TrackNamespace: "tracknamespace",
			},
			buf:    []byte{0x0a, 0x0b},
			expect: []byte{0x0a, 0x0b, byte(AnnounceOkMessageType), 0x0e, 't', 'r', 'a', 'c', 'k', 'n', 'a', 'm', 'e', 's', 'p', 'a', 'c', 'e'},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.aom.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceOkMessage(t *testing.T) {
	cases := []struct {
		r      MessageReader
		len    int
		expect *AnnounceOkMessage
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
			expect: &AnnounceOkMessage{
				TrackNamespace: "tracknamespace",
			},
			err: nil,
		},
		{
			r:   bytes.NewReader([]byte("tracknamespace")),
			len: 5,
			expect: &AnnounceOkMessage{
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
		sem    SubscribeErrorMessage
		buf    []byte
		expect []byte
	}{
		{
			sem: SubscribeErrorMessage{
				FullTrackName: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
			buf: []byte{},
			expect: []byte{
				byte(SubscribeErrorMessageType), 0x03, 0x00, 0x00, 0x00,
			},
		},
		{
			sem: SubscribeErrorMessage{
				FullTrackName: "trackname",
				ErrorCode:     1,
				ReasonPhrase:  "reason",
			},
			buf:    []byte{},
			expect: append(append([]byte{byte(SubscribeErrorMessageType), 0x12, 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
		{
			sem: SubscribeErrorMessage{
				FullTrackName: "trackname",
				ErrorCode:     1,
				ReasonPhrase:  "reason",
			},
			buf:    []byte{0x0a, 0x0b, 0x0c, 0x0d},
			expect: append(append([]byte{0x0a, 0x0b, 0x0c, 0x0d, byte(SubscribeErrorMessageType), 0x12, 0x09}, "trackname"...), append([]byte{0x01, 0x06}, "reason"...)...),
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.sem.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseAnnounceErrorMessage(t *testing.T) {
	cases := []struct {
		r      MessageReader
		len    int
		expect *AnnounceErrorMessage
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
			expect: &AnnounceErrorMessage{
				TrackNamspace: "tracknamespace",
				ErrorCode:     1,
				ReasonPhrase:  "reason phrase",
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
		gam    GoAwayMessage
		buf    []byte
		expect []byte
	}{
		{
			gam: GoAwayMessage{},
			buf: []byte{},
			expect: []byte{
				byte(GoAwayMessageType), 0x00,
			},
		},
		{
			gam: GoAwayMessage{},
			buf: []byte{0x0a, 0x0b},
			expect: []byte{
				0x0a, 0x0b, byte(GoAwayMessageType), 0x00,
			},
		},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			res := tc.gam.Append(tc.buf)
			assert.Equal(t, tc.expect, res)
		})
	}
}

func TestParseGoAwayMessage(t *testing.T) {
	cases := []struct {
		expect *GoAwayMessage
		len    int
		err    error
	}{
		{
			expect: &GoAwayMessage{},
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
