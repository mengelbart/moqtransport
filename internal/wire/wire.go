package wire

type Setup struct {
	Options []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *Setup) Type() ControlMessageType {
	return ControlMessageTypeSetup
}

type GoAway struct {
	NewSessionURI string `proto:"tlv_string"`
	Timeout       uint64 `proto:"varint"`
	RequestID     uint64 `proto:"varint"`
}

func (m *GoAway) Type() ControlMessageType {
	return ControlMessageTypeGoAway
}

type Subscribe struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	TrackName      []byte         `proto:"tlv_bytes"`
	Parameters     []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *Subscribe) Type() ControlMessageType {
	return ControlMessageTypeSubscribe
}

type SubscribeOk struct {
	TrackAlias uint64         `proto:"varint"`
	Parameters []KeyValuePair `proto:"moq_kvp_list"`
	Properties []KeyValuePair `proto:"moq_kvp_list"` // TODO: kvp list without length?
}

func (m *SubscribeOk) Type() ControlMessageType {
	return ControlMessageTypeSubscribeOk
}

type Publish struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	TrackName      []byte         `proto:"tlv_bytes"`
	TrackAlias     uint64         `proto:"varint"`
	Parameters     []KeyValuePair `proto:"moq_kvp_list"`
	Properties     []KeyValuePair `proto:"moq_kvp_list"` // TODO: kvp list without length?
}

func (m *Publish) Type() ControlMessageType {
	return ControlMessageTypePublish
}

type PublishOk struct {
	Parameters []KeyValuePair `proto:"moq_kvp_list"`
	Properties []KeyValuePair `proto:"moq_kvp_list"` // TODO: kvp list without length?
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

type Fetch struct {
	RequestID uint64 `proto:"varint"`
	FetchType uint64 `proto:"varint"`
	// TODO: Fetch versions
	Parameters []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *Fetch) Type() ControlMessageType {
	return ControlMessageTypeFetch
}

type FetchOk struct {
	EndOfTrack  bool           `proto:"bool"`
	EndLocation Location       `proto:"moq_location"`
	Parameters  []KeyValuePair `proto:"moq_kvp_list"`
	Properties  []KeyValuePair `proto:"moq_kvp_list"` // TODO: kvp list without length?
}

func (m *FetchOk) Type() ControlMessageType {
	return ControlMessageTypeFetchOk
}

type TrackStatus struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	TrackName      []byte         `proto:"tlv_bytes"`
	Parameters     []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *TrackStatus) Type() ControlMessageType {
	return ControlMessageTypeTrackStatus
}

type PublishNamespace struct {
	RequestID      uint64         `proto:"varint"`
	TrackNamespace [][]byte       `proto:"ntlv_bytes"`
	Parameters     []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *PublishNamespace) Type() ControlMessageType {
	return ControlMessageTypePublishNamespace
}

type SubscribeNamespace struct {
	RequestID            uint64         `proto:"varint"`
	TrackNamespacePrefix [][]byte       `proto:"ntlv_bytes"`
	Parameters           []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *SubscribeNamespace) Type() ControlMessageType {
	return ControlMessageTypeSubscribeNamespace
}

type SubscribeTracks struct {
	RequestID            uint64         `proto:"varint"`
	TrackNamespacePrefix [][]byte       `proto:"ntlv_bytes"`
	Parameters           []KeyValuePair `proto:"moq_kvp_list"`
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
	Parameters []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *RequestUpdate) Type() ControlMessageType {
	return ControlMessageTypeRequestUpdate
}

type RequestOk struct {
	Parameters []KeyValuePair `proto:"moq_kvp_list"`
	Properties []KeyValuePair `proto:"moq_kvp_list"`
}

func (m *RequestOk) Type() ControlMessageType {
	return ControlMessageTypeRequestOk
}

type RequestError struct {
	ErrorCode     uint64 `proto:"varint"`
	RetryInterval uint64 `proto:"varint"`
	ErrorReason   string `proto:"tlv_string"`
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
