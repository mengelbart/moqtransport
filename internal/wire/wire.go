package wire

import "fmt"

type Setup struct {
	Options []KeyValuePair `proto:"message_list"`
}

func (m *Setup) Type() ControlMessageType {
	return ControlMessageTypeSetup
}

type GoAwayCtrl struct {
	NewSessionURI string `proto:"tlv_string"`
	Timeout       uint64 `proto:"varint"`
	RequestID     uint64 `proto:"varint"`
}

func (m *GoAwayCtrl) Type() ControlMessageType {
	return ControlMessageTypeGoAway
}

type GoAwayReq struct {
	NewSessionURI string `proto:"tlv_string"`
	Timeout       uint64 `proto:"varint"`
}

func (m *GoAwayReq) Type() ControlMessageType {
	return ControlMessageTypeGoAway
}

type Subscribe struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	TrackName      []byte         `proto:"tlv_bytes"`
	Parameters     []KeyValuePair `proto:"message_list"`
}

func (m *Subscribe) Type() ControlMessageType {
	return ControlMessageTypeSubscribe
}

type SubscribeOk struct {
	TrackAlias uint64         `proto:"varint"`
	Parameters []KeyValuePair `proto:"message_list"`
	Properties []KeyValuePair `proto:"message_list_no_length"`
}

func (m *SubscribeOk) Type() ControlMessageType {
	return ControlMessageTypeSubscribeOk
}

type Publish struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	TrackName      []byte         `proto:"tlv_bytes"`
	TrackAlias     uint64         `proto:"varint"`
	Parameters     []KeyValuePair `proto:"message_list"`
	Properties     []KeyValuePair `proto:"message_list_no_length"`
}

func (m *Publish) Type() ControlMessageType {
	return ControlMessageTypePublish
}

type PublishOk struct {
	Parameters []KeyValuePair `proto:"message_list"`
	Properties []KeyValuePair `proto:"message_list_no_length"`
}

func (m *PublishOk) Type() ControlMessageType {
	return ControlMessageTypePublishOk
}

type PublishDone struct {
	StatusCode  uint64 `proto:"varint"`
	StreamCount uint64 `proto:"varint"`
	ErrorReason string `proto:"tlv_string"`
}

func (m *PublishDone) Type() ControlMessageType {
	return ControlMessageTypePublishDone
}

const (
	FetchTypeStandalone      uint64 = 0x1
	FetchTypeRelativeJoining uint64 = 0x2
	FetchTypeAbsoluteJoining uint64 = 0x3
)

type Fetch struct {
	RequestID uint64 `proto:"varint"`
	FetchType uint64 `proto:"varint"`

	TrackNamespace [][]byte `proto:"ntlv_bytes,if=isStandalone"`
	TrackName      []byte   `proto:"tlv_bytes,if=isStandalone"`
	StartLocation  Location `proto:"message,if=isStandalone"`
	EndLocation    Location `proto:"message,if=isStandalone"`

	JoiningRequestID uint64 `proto:"varint,if=isJoining"`
	JoiningStart     uint64 `proto:"varint,if=isJoining"`

	Parameters []KeyValuePair `proto:"message_list"`
}

func (m *Fetch) Type() ControlMessageType {
	return ControlMessageTypeFetch
}

func (m *Fetch) isStandalone() bool {
	return m.FetchType == FetchTypeStandalone
}

func (m *Fetch) isJoining() bool {
	return m.FetchType == FetchTypeRelativeJoining || m.FetchType == FetchTypeAbsoluteJoining
}

// validate rejects a fetch type that carries neither structure.
func (m *Fetch) validate() error {
	if !m.isStandalone() && !m.isJoining() {
		return fmt.Errorf("invalid fetch type: %d", m.FetchType)
	}
	return nil
}

type FetchOk struct {
	EndOfTrack  bool           `proto:"bool"`
	EndLocation Location       `proto:"message"`
	Parameters  []KeyValuePair `proto:"message_list"`
	Properties  []KeyValuePair `proto:"message_list_no_length"`
}

func (m *FetchOk) Type() ControlMessageType {
	return ControlMessageTypeFetchOk
}

type TrackStatus struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	TrackName      []byte         `proto:"tlv_bytes"`
	Parameters     []KeyValuePair `proto:"message_list"`
}

func (m *TrackStatus) Type() ControlMessageType {
	return ControlMessageTypeTrackStatus
}

type PublishNamespace struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	Parameters     []KeyValuePair `proto:"message_list"`
}

func (m *PublishNamespace) Type() ControlMessageType {
	return ControlMessageTypePublishNamespace
}

type SubscribeNamespace struct {
	RequestID            uint64         `proto:"varint"`
	TrackNamespacePrefix [][]byte       `proto:"ntlv_bytes"`
	Parameters           []KeyValuePair `proto:"message_list"`
}

func (m *SubscribeNamespace) Type() ControlMessageType {
	return ControlMessageTypeSubscribeNamespace
}

type SubscribeTracks struct {
	RequestID            uint64         `proto:"varint"`
	TrackNamespacePrefix [][]byte       `proto:"ntlv_bytes"`
	Parameters           []KeyValuePair `proto:"message_list"`
}

func (m *SubscribeTracks) Type() ControlMessageType {
	return ControlMessageTypeSubscribeTracks
}

type Namespace struct {
	TrackNamespaceSuffix [][]byte `proto:"ntlv_bytes"`
}

func (m *Namespace) Type() ControlMessageType {
	return ControlMessageTypeNamespace
}

type NamespaceDone struct {
	TrackNamespaceSuffix [][]byte `proto:"ntlv_bytes"`
}

func (m *NamespaceDone) Type() ControlMessageType {
	return ControlMessageTypeNamespaceDone
}

type PublishBlocked struct {
	TrackNamespaceSuffix [][]byte `proto:"ntlv_bytes"`
	TrackName            []byte   `proto:"tlv_bytes"`
}

func (m *PublishBlocked) Type() ControlMessageType {
	return ControlMessageTypePublishBlocked
}

type RequestUpdate struct {
	RequestID  uint64         `proto:"varint"`
	Parameters []KeyValuePair `proto:"message_list"`
}

func (m *RequestUpdate) Type() ControlMessageType {
	return ControlMessageTypeRequestUpdate
}

type RequestOk struct {
	Parameters []KeyValuePair `proto:"message_list"`
	Properties []KeyValuePair `proto:"message_list_no_length"`
}

func (m *RequestOk) Type() ControlMessageType {
	return ControlMessageTypeRequestOk
}

type RequestError struct {
	ErrorCode     uint64 `proto:"varint"`
	RetryInterval uint64 `proto:"varint"`
	ErrorReason   string `proto:"tlv_string"`
	// TODO: Implement Redirect
}

func (m *RequestError) Type() ControlMessageType {
	return ControlMessageTypeRequestError
}

type FetchHeader struct {
	RequestID uint64 `proto:"varint"`
}

func (m *FetchHeader) Type() ControlMessageType {
	return ControlMessageTypeFetchHeader
}

type Padding struct {
}

func (m *Padding) Type() ControlMessageType {
	return ControlMessageTypePadding
}
