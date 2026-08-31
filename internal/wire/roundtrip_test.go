package wire

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/mengelbart/moqtransport/varint"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kvps() []KeyValuePair {
	return []KeyValuePair{
		{Type: 2, Varint: 4200},
		{Type: 3, Bytes: []byte("value")},
	}
}

func roundTripCases() []struct {
	name       string
	streamType StreamType
	msg        ControlMessage
} {
	return []struct {
		name       string
		streamType StreamType
		msg        ControlMessage
	}{
		{"Setup", StreamTypeControl, &Setup{Options: kvps()}},
		{"GoAwayCtrl", StreamTypeControl, &GoAwayCtrl{NewSessionURI: "moqt://example", Timeout: 17, RequestID: 3}},

		{"GoAwayReq", StreamTypeRequest, &GoAwayReq{NewSessionURI: "moqt://example", Timeout: 17}},
		{"Subscribe", StreamTypeRequest, &Subscribe{
			RequestID:      7,
			TrackNamespace: [][]byte{[]byte("ns"), []byte("sub")},
			TrackName:      []byte("track"),
			Parameters:     kvps(),
		}},
		{"SubscribeOk", StreamTypeRequest, &SubscribeOk{TrackAlias: 9, Parameters: kvps(), Properties: kvps()}},
		{"Publish", StreamTypeRequest, &Publish{
			RequestID:      7,
			TrackNamespace: [][]byte{[]byte("ns")},
			TrackName:      []byte("track"),
			TrackAlias:     9,
			Parameters:     kvps(),
			Properties:     kvps(),
		}},
		{"PublishOk", StreamTypeRequest, &PublishOk{Parameters: kvps(), Properties: kvps()}},
		{"PublishDone", StreamTypeRequest, &PublishDone{StatusCode: 2, StreamCount: 11, ErrorReason: "done"}},
		{"FetchStandalone", StreamTypeRequest, &Fetch{
			RequestID:      7,
			FetchType:      FetchTypeStandalone,
			TrackNamespace: [][]byte{[]byte("ns"), []byte("sub")},
			TrackName:      []byte("track"),
			StartLocation:  Location{Group: 1, Object: 2},
			EndLocation:    Location{Group: 3, Object: 4},
			Parameters:     kvps(),
		}},
		{"FetchRelativeJoining", StreamTypeRequest, &Fetch{
			RequestID:        7,
			FetchType:        FetchTypeRelativeJoining,
			JoiningRequestID: 3,
			JoiningStart:     2,
			Parameters:       kvps(),
		}},
		{"FetchAbsoluteJoining", StreamTypeRequest, &Fetch{
			RequestID:        7,
			FetchType:        FetchTypeAbsoluteJoining,
			JoiningRequestID: 3,
			JoiningStart:     9,
			Parameters:       kvps(),
		}},
		{"FetchOk", StreamTypeRequest, &FetchOk{
			EndOfTrack:  true,
			EndLocation: Location{Group: 4, Object: 5},
			Parameters:  kvps(),
			Properties:  kvps(),
		}},
		{"TrackStatus", StreamTypeRequest, &TrackStatus{
			RequestID:      7,
			TrackNamespace: [][]byte{[]byte("ns")},
			TrackName:      []byte("track"),
			Parameters:     kvps(),
		}},
		{"PublishNamespace", StreamTypeRequest, &PublishNamespace{
			RequestID:      7,
			TrackNamespace: [][]byte{[]byte("ns")},
			Parameters:     kvps(),
		}},
		{"SubscribeNamespace", StreamTypeRequest, &SubscribeNamespace{
			RequestID:            7,
			TrackNamespacePrefix: [][]byte{[]byte("ns")},
			Parameters:           kvps(),
		}},
		{"SubscribeTracks", StreamTypeRequest, &SubscribeTracks{
			RequestID:            7,
			TrackNamespacePrefix: [][]byte{[]byte("ns")},
			Parameters:           kvps(),
		}},
		{"Namespace", StreamTypeRequest, &Namespace{TrackNamespaceSuffix: [][]byte{[]byte("a"), []byte("b")}}},
		{"NamespaceDone", StreamTypeRequest, &NamespaceDone{TrackNamespaceSuffix: [][]byte{[]byte("a")}}},
		{"PublishBlocked", StreamTypeRequest, &PublishBlocked{
			TrackNamespaceSuffix: [][]byte{[]byte("a")},
			TrackName:            []byte("track"),
		}},
		{"RequestUpdate", StreamTypeRequest, &RequestUpdate{RequestID: 7, Parameters: kvps()}},
		{"RequestOk", StreamTypeRequest, &RequestOk{Parameters: kvps(), Properties: kvps()}},
		{"RequestError", StreamTypeRequest, &RequestError{ErrorCode: 3, RetryInterval: 8, ErrorReason: "nope"}},

		{"FetchHeader", StreamTypeData, &FetchHeader{RequestID: 42}},
		{"Padding", StreamTypeData, &Padding{}},
		{"SubgroupHeader", StreamTypeData, NewSubgroupHeader(4, 7, 9, 200)},
	}
}

func encode(t *testing.T, msg ControlMessage) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, NewAppender(&buf, 18).Write(msg))
	return buf.Bytes()
}

func TestMessageRoundTrip(t *testing.T) {
	for _, tc := range roundTripCases() {
		t.Run(tc.name, func(t *testing.T) {
			encoded := encode(t, tc.msg)

			got, err := NewParser(bytes.NewReader(encoded), 18, tc.streamType).Read()
			require.NoError(t, err)
			assert.Equal(t, tc.msg, got)
		})
	}
}

func deltaPairs() []KeyValuePair {
	return []KeyValuePair{
		{Type: 2, Varint: 4200},
		{Type: 77, Bytes: []byte("value")},
		{Type: 77, Bytes: []byte("other")},
	}
}

// deltaPairBytes is the delta encoding of deltaPairs, without any framing.
var deltaPairBytes = []byte{
	0x02, 0x90, 0x68, // delta 2, type 2: varint 4200
	0x4b, 0x05, 'v', 'a', 'l', 'u', 'e', // delta 75, type 77: five bytes
	0x00, 0x05, 'o', 't', 'h', 'e', 'r', // delta 0, type 77: five bytes
}

func TestSetupOptionsBytes(t *testing.T) {
	msg := &Setup{Options: deltaPairs()}

	assert.Equal(t, deltaPairBytes, msg.append_v18(nil))

	got, err := NewParser(bytes.NewReader(encode(t, msg)), 18, StreamTypeControl).Read()
	require.NoError(t, err)
	assert.Equal(t, msg, got)
}

func TestParameterListBytes(t *testing.T) {
	msg := &RequestUpdate{RequestID: 7, Parameters: deltaPairs()}

	want := []byte{0x07, 0x03} // request ID, number of parameters
	want = append(want, deltaPairBytes...)
	assert.Equal(t, want, msg.append_v18(nil))

	got, err := NewParser(bytes.NewReader(encode(t, msg)), 18, StreamTypeRequest).Read()
	require.NoError(t, err)
	assert.Equal(t, msg, got)
}

// TestMessageTruncated cuts every encoding at every prefix. io.EOF means only
// that no byte of a message was read, so it is expected at offset 0 and nowhere
// else: once a message has begun, ending early is io.ErrUnexpectedEOF.
func TestMessageTruncated(t *testing.T) {
	for _, tc := range roundTripCases() {
		t.Run(tc.name, func(t *testing.T) {
			encoded := encode(t, tc.msg)
			for i := range encoded {
				want := io.ErrUnexpectedEOF
				if i == 0 {
					want = io.EOF
				}
				_, err := NewParser(bytes.NewReader(encoded[:i]), 18, tc.streamType).Read()
				assert.ErrorIs(t, err, want, fmt.Sprintf("truncated to %v of %v bytes", i, len(encoded)))
			}
		})
	}
}

// TestParseBodyLongerThanMessage rejects a body the parser under-reads, whose
// leftover bytes would otherwise stay in the stream and desync every message
// after it.
func TestParseBodyLongerThanMessage(t *testing.T) {
	msg := &GoAwayReq{NewSessionURI: "moqt://example", Timeout: 17}
	body := msg.append_v18(nil)

	encoded := varint.Append(nil, uint64(msg.Type()))
	encoded = append(encoded, byte((len(body)+1)>>8), byte(len(body)+1))
	encoded = append(encoded, body...)
	encoded = append(encoded, 0xff)

	_, err := NewParser(bytes.NewReader(encoded), 18, StreamTypeRequest).Read()
	assert.ErrorIs(t, err, errLengthMismatch)
}

// TestParseFetchUnknownType covers the one conditional whose discriminator is a
// field of the message rather than a bit of the type varint: an unknown fetch
// type carries neither structure, and must not be read as if it carried the
// standalone one.
func TestParseFetchUnknownType(t *testing.T) {
	standalone := &Fetch{
		RequestID:      7,
		FetchType:      FetchTypeStandalone,
		TrackNamespace: [][]byte{[]byte("ns")},
		TrackName:      []byte("track"),
		StartLocation:  Location{Group: 1, Object: 2},
		EndLocation:    Location{Group: 3, Object: 4},
		Parameters:     kvps(),
	}
	encoded := encode(t, standalone)

	// Rewrite the fetch type in place. It is the second varint of the body,
	// after the one byte request ID.
	body := len(varint.Append(nil, uint64(standalone.Type()))) + 2
	require.Equal(t, byte(FetchTypeStandalone), encoded[body+1])
	encoded[body+1] = 0x04

	_, err := NewParser(bytes.NewReader(encoded), 18, StreamTypeRequest).Read()
	assert.Error(t, err)
}
