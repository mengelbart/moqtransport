package transport

import (
	"errors"
	"io"
	"log"
	"time"

	"gitlab.lrz.de/cm/moqtransport/varint"
)

var (
	errInvalidMessageReader   = errors.New("invalid message reader")
	errInvalidMessageEncoding = errors.New("invalid message encoding")
)

type role int

const (
	serverRole role = iota
	clientRole
)

type Message interface {
	String() string
	Append([]byte) []byte
}

type MessageType uint64

const (
	ObjectMessageType MessageType = iota
	SetupMessageType
	Reserved0x02Type
	SubscribeRequestMessageType
	SubscribeOkMessageType
	SubscribeErrorMessageType
	AnnounceMessageType
	AnnounceOkMessageType
	AnnounceErrorMessageType
	Reserved0x09Type
	GoAwayMessageType
)

func (mt MessageType) String() string {
	switch mt {
	case ObjectMessageType:
		return "ObjectMessage"
	case SetupMessageType:
		return "SetupMessage"
	case Reserved0x02Type:
		return "Reserved0x02Type"
	case SubscribeRequestMessageType:
		return "SubscribeRequestMessage"
	case SubscribeOkMessageType:
		return "SubscribeOkMessage"
	case SubscribeErrorMessageType:
		return "SubscribeErrorMessage"
	case AnnounceMessageType:
		return "AnnounceMessage"
	case AnnounceOkMessageType:
		return "AnnounceOkMessage"
	case AnnounceErrorMessageType:
		return "AnnounceErrorMessage"
	case Reserved0x09Type:
		return "Reserved0x09Type"
	case GoAwayMessageType:
		return "GoAwayMessage"
	}
	return "unknown message type"
}

const (
	objectMessageMinimumLength           = 4
	clientSetupMessageMinimumLength      = 2
	serverSetupMessageMinimumLength      = 1
	subscribeRequestMessageMinimumLength = 1
	subscribeOkMessageMinimumLength      = 3
	subscribeErrorMessageMinimumLength   = 3
	announceMessageMinimumLength         = 1
	announceOkMessageMinimumLength       = 0
	announceErrorMessageMinimumLength    = 3
	goAwayMessageMinimumLength           = 0
)

type MessageReader interface {
	io.Reader
	io.ByteReader
}

func ReadNext(reader MessageReader, r role) (Message, error) {
	mt, err := varint.Read(reader)
	if err != nil {
		return nil, err
	}
	l, err := varint.Read(reader)
	if err != nil {
		return nil, err
	}
	length := int(l)
	log.Printf("parsing message of type: %v\n", MessageType(mt))

	switch MessageType(mt) {
	case ObjectMessageType:
		msg, err := parseObjectMessage(reader, length)
		return msg, err
	case SetupMessageType:
		switch r {
		case serverRole:
			csm, err := parseClientSetupMessage(reader, length)
			return csm, err
		case clientRole:
			ssm, err := parseServerSetupMessage(reader, length)
			return ssm, err
		}
	case SubscribeRequestMessageType:
		srm, err := parseSubscribeRequestMessage(reader, length)
		return srm, err
	case SubscribeOkMessageType:
		// TODO: Use mockable time.Now()?
		som, err := parseSubscribeOkMessage(reader, length)
		return som, err
	case SubscribeErrorMessageType:
		sem, err := parseSubscribeErrorMessage(reader, length)
		return sem, err
	case AnnounceMessageType:
		am, err := parseAnnounceMessage(reader, length)
		return am, err
	case AnnounceOkMessageType:
		aom, err := parseAnnounceOkMessage(reader, length)
		return aom, err
	case AnnounceErrorMessageType:
		return parseAnnounceErrorMessage(reader, length)
	case GoAwayMessageType:
		return parseGoAwayMessage(length)
	}
	return nil, errors.New("unknown message type")
}

type ObjectMessage struct {
	TrackID         uint64
	GroupSequence   uint64
	ObjectSequence  uint64
	ObjectSendOrder uint64
	ObjectPayload   []byte
}

func (m ObjectMessage) String() string {
	return "ObjectMessage"
}

func (m *ObjectMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(ObjectMessageType))
	buf = varint.Append(buf,
		uint64(varint.Len(m.TrackID))+
			uint64(varint.Len(m.GroupSequence))+
			uint64(varint.Len(m.ObjectSequence))+
			uint64(varint.Len(m.ObjectSendOrder))+
			uint64(len(m.ObjectPayload)),
	)

	buf = varint.Append(buf, m.TrackID)
	buf = varint.Append(buf, m.GroupSequence)
	buf = varint.Append(buf, m.ObjectSequence)
	buf = varint.Append(buf, m.ObjectSendOrder)
	buf = append(buf, m.ObjectPayload...)
	return buf
}

func parseObjectMessage(r MessageReader, length int) (*ObjectMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < objectMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	offset := 0
	trackID, n, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	offset += n
	groupSequence, n, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	offset += n
	objectSequence, n, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	offset += n
	objectSendOrder, n, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	if length == 0 {
		var objectPayload []byte
		objectPayload, err = io.ReadAll(r)
		return &ObjectMessage{
			TrackID:         trackID,
			GroupSequence:   groupSequence,
			ObjectSequence:  objectSequence,
			ObjectSendOrder: objectSendOrder,
			ObjectPayload:   objectPayload,
		}, err
	}
	offset += n
	payloadLen := length - offset
	if payloadLen > 0 {
		objectPayload := make([]byte, payloadLen)
		_, err = io.ReadFull(r, objectPayload)
		if err != nil {
			return nil, err
		}
		return &ObjectMessage{
			TrackID:         trackID,
			GroupSequence:   groupSequence,
			ObjectSequence:  objectSequence,
			ObjectSendOrder: objectSendOrder,
			ObjectPayload:   objectPayload,
		}, err
	}
	return &ObjectMessage{
		TrackID:         trackID,
		GroupSequence:   groupSequence,
		ObjectSequence:  objectSequence,
		ObjectSendOrder: objectSendOrder,
		ObjectPayload:   []byte{},
	}, err
}

type ClientSetupMessage struct {
	SupportedVersions Versions
	SetupParameters   Parameters
}

func (m ClientSetupMessage) String() string {
	return "ClientSetupMessage"
}

func (m *ClientSetupMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(SetupMessageType))
	buf = varint.Append(buf,
		uint64(varint.Len(uint64(len(m.SupportedVersions))))+
			m.SupportedVersions.Len()+
			m.SetupParameters.Len(),
	)
	buf = varint.Append(buf, uint64(len(m.SupportedVersions)))
	for _, v := range m.SupportedVersions {
		buf = varint.Append(buf, uint64(v))
	}
	for _, p := range m.SetupParameters {
		buf = p.Append(buf)
	}
	return buf
}

func parseClientSetupMessage(r MessageReader, length int) (*ClientSetupMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < clientSetupMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	offset := 0
	numSupportedVersions, n, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	offset += n
	var versions Versions
	for i := 0; i < int(numSupportedVersions); i++ {
		var v uint64
		v, n, err = varint.ReadWithLen(r)
		if err != nil {
			return nil, err
		}
		offset += n
		versions = append(versions, Version(v))
	}
	ps, err := parseParameters(r, length-offset)
	if err != nil {
		return nil, err
	}
	return &ClientSetupMessage{
		SupportedVersions: versions,
		SetupParameters:   ps,
	}, nil
}

type ServerSetupMessage struct {
	SelectedVersion Version
	SetupParameters Parameters
}

func (m ServerSetupMessage) String() string {
	return "ServerSetupMessage"
}

func (m *ServerSetupMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(SetupMessageType))
	buf = varint.Append(buf,
		m.SelectedVersion.Len()+
			m.SetupParameters.Len(),
	)
	buf = varint.Append(buf, uint64(m.SelectedVersion))
	for _, p := range m.SetupParameters {
		buf = p.Append(buf)
	}
	return buf
}

func parseServerSetupMessage(r MessageReader, length int) (*ServerSetupMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < serverSetupMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	sv, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r, length-1)
	if err != nil {
		return nil, err
	}
	return &ServerSetupMessage{
		SelectedVersion: Version(sv),
		SetupParameters: ps,
	}, nil
}

type SubscribeRequestMessage struct {
	FullTrackName          string
	TrackRequestParameters Parameters
}

func (m SubscribeRequestMessage) String() string {
	return "SubscribeRequestMessage"
}

func (m SubscribeRequestMessage) key() messageKey {
	return messageKey{
		mt: SubscribeRequestMessageType,
		id: m.FullTrackName,
	}
}

func (m *SubscribeRequestMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(SubscribeRequestMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.FullTrackName)+
			m.TrackRequestParameters.Len(),
	)
	buf = appendVarIntString(buf, m.FullTrackName)
	for _, p := range m.TrackRequestParameters {
		buf = p.Append(buf)
	}
	return buf
}

func parseSubscribeRequestMessage(r MessageReader, length int) (*SubscribeRequestMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < subscribeRequestMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	name, n, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r, length-n)
	if err != nil {
		return nil, err
	}
	return &SubscribeRequestMessage{
		FullTrackName:          name,
		TrackRequestParameters: ps,
	}, nil
}

type SubscribeOkMessage struct {
	FullTrackName string
	TrackID       uint64
	Expires       time.Duration
}

func (m SubscribeOkMessage) String() string {
	return "SubscribeOkMessage"
}

func (m SubscribeOkMessage) key() messageKey {
	return messageKey{
		mt: SubscribeRequestMessageType,
		id: m.FullTrackName,
	}
}

func (m *SubscribeOkMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(SubscribeOkMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.FullTrackName)+
			uint64(varint.Len(m.TrackID))+
			uint64(varint.Len(uint64(m.Expires))),
	)
	buf = appendVarIntString(buf, m.FullTrackName)
	buf = varint.Append(buf, m.TrackID)
	buf = varint.Append(buf, uint64(m.Expires))
	return buf
}

func parseSubscribeOkMessage(r MessageReader, length int) (*SubscribeOkMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < subscribeOkMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	fullTrackName, n, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	trackID, m, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	e, k, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	if n+m+k != length {
		return nil, errInvalidMessageEncoding
	}
	return &SubscribeOkMessage{
		FullTrackName: fullTrackName,
		TrackID:       trackID,
		Expires:       time.Duration(e) * time.Millisecond,
	}, nil
}

type SubscribeErrorMessage struct {
	FullTrackName string
	ErrorCode     uint64
	ReasonPhrase  string
}

func (m SubscribeErrorMessage) String() string {
	return "SubscribeErrorMessage"
}

func (m SubscribeErrorMessage) key() messageKey {
	return messageKey{
		mt: SubscribeRequestMessageType,
		id: m.FullTrackName,
	}
}

func (m *SubscribeErrorMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(SubscribeErrorMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.FullTrackName)+
			uint64(varint.Len(m.ErrorCode))+
			varIntStringLen(m.ReasonPhrase),
	)
	buf = appendVarIntString(buf, m.FullTrackName)
	buf = varint.Append(buf, m.ErrorCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
	return buf
}

func parseSubscribeErrorMessage(r MessageReader, length int) (*SubscribeErrorMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < subscribeErrorMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	fullTrackName, n, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	errorCode, m, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, k, err := parseVarIntString(r)
	if n+m+k != length {
		return nil, errInvalidMessageEncoding
	}
	return &SubscribeErrorMessage{
		FullTrackName: fullTrackName,
		ErrorCode:     errorCode,
		ReasonPhrase:  reasonPhrase,
	}, err
}

type AnnounceMessage struct {
	TrackNamespace  string
	TrackParameters Parameters
}

func (m AnnounceMessage) String() string {
	return "AnnounceMessage"
}

func (m AnnounceMessage) key() messageKey {
	return messageKey{
		mt: AnnounceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *AnnounceMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(AnnounceMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.TrackNamespace)+
			m.TrackParameters.Len(),
	)
	buf = appendVarIntString(buf, m.TrackNamespace)
	for _, p := range m.TrackParameters {
		buf = p.Append(buf)
	}
	return buf
}

func parseAnnounceMessage(r MessageReader, length int) (*AnnounceMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < announceMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	var n int
	trackNamspace, n, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r, length-n)
	if err != nil {
		return nil, err
	}
	return &AnnounceMessage{
		TrackNamespace:  trackNamspace,
		TrackParameters: ps,
	}, nil
}

type AnnounceOkMessage struct {
	TrackNamespace string
}

func (m AnnounceOkMessage) String() string {
	return "AnnounceOkMessage"
}

func (m AnnounceOkMessage) key() messageKey {
	return messageKey{
		mt: AnnounceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *AnnounceOkMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(AnnounceOkMessageType))
	buf = varint.Append(buf, uint64(len(m.TrackNamespace)))
	buf = append(buf, []byte(m.TrackNamespace)...)
	return buf
}

func parseAnnounceOkMessage(r MessageReader, length int) (*AnnounceOkMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length > 0 {
		buf := make([]byte, length)
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return nil, err
		}
		return &AnnounceOkMessage{
			TrackNamespace: string(buf),
		}, err
	}
	name, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &AnnounceOkMessage{
		TrackNamespace: string(name),
	}, err
}

type AnnounceErrorMessage struct {
	TrackNamespace string
	ErrorCode      uint64
	ReasonPhrase   string
}

func (m AnnounceErrorMessage) String() string {
	return "AnnounceErrorMessage"
}

func (m AnnounceErrorMessage) key() messageKey {
	return messageKey{
		mt: AnnounceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *AnnounceErrorMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(AnnounceErrorMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.TrackNamespace)+
			uint64(varint.Len(m.ErrorCode))+
			varIntStringLen(m.ReasonPhrase),
	)
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = varint.Append(buf, m.ErrorCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
	return buf
}

func parseAnnounceErrorMessage(r MessageReader, length int) (*AnnounceErrorMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length < announceErrorMessageMinimumLength {
		return nil, errInvalidMessageEncoding
	}
	trackNamspace, n, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	errorCode, m, err := varint.ReadWithLen(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, k, err := parseVarIntString(r)
	if n+m+k != length {
		return nil, errInvalidMessageEncoding
	}
	return &AnnounceErrorMessage{
		TrackNamespace: trackNamspace,
		ErrorCode:      errorCode,
		ReasonPhrase:   reasonPhrase,
	}, err
}

type GoAwayMessage struct {
}

func (m GoAwayMessage) String() string {
	return "GoAwayMessage"
}

func (m *GoAwayMessage) Append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(GoAwayMessageType))
	buf = varint.Append(buf, 0)
	return buf
}

func parseGoAwayMessage(length int) (*GoAwayMessage, error) {
	if length != 0 {
		return nil, errInvalidMessageEncoding
	}
	return &GoAwayMessage{}, nil
}
