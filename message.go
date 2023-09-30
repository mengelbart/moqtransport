package moqtransport

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

type message interface {
	String() string
	append([]byte) []byte
}

type messageType uint64

const (
	objectMessageType messageType = iota
	setupMessageType
	reserved0x02Type
	subscribeRequestMessageType
	subscribeOkMessageType
	subscribeErrorMessageType
	announceMessageType
	announceOkMessageType
	announceErrorMessageType
	reserved0x09Type
	goAwayMessageType
)

func (mt messageType) String() string {
	switch mt {
	case objectMessageType:
		return "ObjectMessage"
	case setupMessageType:
		return "SetupMessage"
	case reserved0x02Type:
		return "Reserved0x02Type"
	case subscribeRequestMessageType:
		return "SubscribeRequestMessage"
	case subscribeOkMessageType:
		return "SubscribeOkMessage"
	case subscribeErrorMessageType:
		return "SubscribeErrorMessage"
	case announceMessageType:
		return "AnnounceMessage"
	case announceOkMessageType:
		return "AnnounceOkMessage"
	case announceErrorMessageType:
		return "AnnounceErrorMessage"
	case reserved0x09Type:
		return "Reserved0x09Type"
	case goAwayMessageType:
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

type messageReader interface {
	io.Reader
	io.ByteReader
}

func readNext(reader messageReader, r role) (message, error) {
	mt, err := varint.Read(reader)
	if err != nil {
		return nil, err
	}
	l, err := varint.Read(reader)
	if err != nil {
		return nil, err
	}
	length := int(l)
	log.Printf("parsing message of type: %v and length: %v\n", messageType(mt), length)

	switch messageType(mt) {
	case objectMessageType:
		msg, err := parseObjectMessage(reader, length)
		return msg, err
	case setupMessageType:
		switch r {
		case serverRole:
			csm, err := parseClientSetupMessage(reader, length)
			return csm, err
		case clientRole:
			ssm, err := parseServerSetupMessage(reader, length)
			return ssm, err
		}
	case subscribeRequestMessageType:
		srm, err := parseSubscribeRequestMessage(reader, length)
		return srm, err
	case subscribeOkMessageType:
		// TODO: Use mockable time.Now()?
		som, err := parseSubscribeOkMessage(reader, length)
		return som, err
	case subscribeErrorMessageType:
		sem, err := parseSubscribeErrorMessage(reader, length)
		return sem, err
	case announceMessageType:
		am, err := parseAnnounceMessage(reader, length)
		return am, err
	case announceOkMessageType:
		aom, err := parseAnnounceOkMessage(reader, length)
		return aom, err
	case announceErrorMessageType:
		return parseAnnounceErrorMessage(reader, length)
	case goAwayMessageType:
		return parseGoAwayMessage(length)
	}
	return nil, errors.New("unknown message type")
}

type objectMessage struct {
	TrackID         uint64
	GroupSequence   uint64
	ObjectSequence  uint64
	ObjectSendOrder uint64
	ObjectPayload   []byte
}

func (m objectMessage) String() string {
	return "ObjectMessage"
}

func (m *objectMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(objectMessageType))
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

func parseObjectMessage(r messageReader, length int) (*objectMessage, error) {
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
		return &objectMessage{
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
		return &objectMessage{
			TrackID:         trackID,
			GroupSequence:   groupSequence,
			ObjectSequence:  objectSequence,
			ObjectSendOrder: objectSendOrder,
			ObjectPayload:   objectPayload,
		}, err
	}
	return &objectMessage{
		TrackID:         trackID,
		GroupSequence:   groupSequence,
		ObjectSequence:  objectSequence,
		ObjectSendOrder: objectSendOrder,
		ObjectPayload:   []byte{},
	}, err
}

type clientSetupMessage struct {
	SupportedVersions versions
	SetupParameters   parameters
}

func (m clientSetupMessage) String() string {
	return "ClientSetupMessage"
}

func (m *clientSetupMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(setupMessageType))
	buf = varint.Append(buf,
		uint64(varint.Len(uint64(len(m.SupportedVersions))))+
			m.SupportedVersions.Len()+
			m.SetupParameters.length(),
	)
	buf = varint.Append(buf, uint64(len(m.SupportedVersions)))
	for _, v := range m.SupportedVersions {
		buf = varint.Append(buf, uint64(v))
	}
	for _, p := range m.SetupParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseClientSetupMessage(r messageReader, length int) (*clientSetupMessage, error) {
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
	var versions versions
	for i := 0; i < int(numSupportedVersions); i++ {
		var v uint64
		v, n, err = varint.ReadWithLen(r)
		if err != nil {
			return nil, err
		}
		offset += n
		versions = append(versions, version(v))
	}
	ps, err := parseParameters(r, length-offset)
	if err != nil {
		return nil, err
	}
	return &clientSetupMessage{
		SupportedVersions: versions,
		SetupParameters:   ps,
	}, nil
}

type serverSetupMessage struct {
	SelectedVersion version
	SetupParameters parameters
}

func (m serverSetupMessage) String() string {
	return "ServerSetupMessage"
}

func (m *serverSetupMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(setupMessageType))
	buf = varint.Append(buf,
		m.SelectedVersion.Len()+
			m.SetupParameters.length(),
	)
	buf = varint.Append(buf, uint64(m.SelectedVersion))
	for _, p := range m.SetupParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseServerSetupMessage(r messageReader, length int) (*serverSetupMessage, error) {
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
	return &serverSetupMessage{
		SelectedVersion: version(sv),
		SetupParameters: ps,
	}, nil
}

type subscribeRequestMessage struct {
	FullTrackName          string
	TrackRequestParameters parameters
}

func (m subscribeRequestMessage) String() string {
	return "SubscribeRequestMessage"
}

func (m subscribeRequestMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: m.FullTrackName,
	}
}

func (m *subscribeRequestMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeRequestMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.FullTrackName)+
			m.TrackRequestParameters.length(),
	)
	buf = appendVarIntString(buf, m.FullTrackName)
	for _, p := range m.TrackRequestParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseSubscribeRequestMessage(r messageReader, length int) (*subscribeRequestMessage, error) {
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
	return &subscribeRequestMessage{
		FullTrackName:          name,
		TrackRequestParameters: ps,
	}, nil
}

type subscribeOkMessage struct {
	FullTrackName string
	TrackID       uint64
	Expires       time.Duration
}

func (m subscribeOkMessage) String() string {
	return "SubscribeOkMessage"
}

func (m subscribeOkMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: m.FullTrackName,
	}
}

func (m *subscribeOkMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeOkMessageType))
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

func parseSubscribeOkMessage(r messageReader, length int) (*subscribeOkMessage, error) {
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
	return &subscribeOkMessage{
		FullTrackName: fullTrackName,
		TrackID:       trackID,
		Expires:       time.Duration(e) * time.Millisecond,
	}, nil
}

type subscribeErrorMessage struct {
	FullTrackName string
	ErrorCode     uint64
	ReasonPhrase  string
}

func (m subscribeErrorMessage) String() string {
	return "SubscribeErrorMessage"
}

func (m subscribeErrorMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: m.FullTrackName,
	}
}

func (m *subscribeErrorMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeErrorMessageType))
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

func parseSubscribeErrorMessage(r messageReader, length int) (*subscribeErrorMessage, error) {
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
	return &subscribeErrorMessage{
		FullTrackName: fullTrackName,
		ErrorCode:     errorCode,
		ReasonPhrase:  reasonPhrase,
	}, err
}

type announceMessage struct {
	TrackNamespace  string
	TrackParameters parameters
}

func (m announceMessage) String() string {
	return "AnnounceMessage"
}

func (m announceMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *announceMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(announceMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.TrackNamespace)+
			m.TrackParameters.length(),
	)
	buf = appendVarIntString(buf, m.TrackNamespace)
	for _, p := range m.TrackParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseAnnounceMessage(r messageReader, length int) (*announceMessage, error) {
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
	return &announceMessage{
		TrackNamespace:  trackNamspace,
		TrackParameters: ps,
	}, nil
}

type announceOkMessage struct {
	TrackNamespace string
}

func (m announceOkMessage) String() string {
	return "AnnounceOkMessage"
}

func (m announceOkMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *announceOkMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(announceOkMessageType))
	buf = varint.Append(buf, uint64(len(m.TrackNamespace)))
	buf = append(buf, []byte(m.TrackNamespace)...)
	return buf
}

func parseAnnounceOkMessage(r messageReader, length int) (*announceOkMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	log.Printf("parsing announce ok of length %v", length)
	if length > 0 {
		buf := make([]byte, length)
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return nil, err
		}
		return &announceOkMessage{
			TrackNamespace: string(buf),
		}, err
	}
	name, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	log.Printf("parsing announce ok done")
	return &announceOkMessage{
		TrackNamespace: string(name),
	}, err
}

type announceErrorMessage struct {
	TrackNamespace string
	ErrorCode      uint64
	ReasonPhrase   string
}

func (m announceErrorMessage) String() string {
	return "AnnounceErrorMessage"
}

func (m announceErrorMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *announceErrorMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(announceErrorMessageType))
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

func parseAnnounceErrorMessage(r messageReader, length int) (*announceErrorMessage, error) {
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
	return &announceErrorMessage{
		TrackNamespace: trackNamspace,
		ErrorCode:      errorCode,
		ReasonPhrase:   reasonPhrase,
	}, err
}

type goAwayMessage struct {
}

func (m goAwayMessage) String() string {
	return "GoAwayMessage"
}

func (m *goAwayMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(goAwayMessageType))
	buf = varint.Append(buf, 0)
	return buf
}

func parseGoAwayMessage(length int) (*goAwayMessage, error) {
	if length != 0 {
		return nil, errInvalidMessageEncoding
	}
	return &goAwayMessage{}, nil
}
