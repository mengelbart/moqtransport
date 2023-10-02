package moqtransport

import (
	"errors"
	"fmt"
	"io"
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
	unannounceMessageType
	goAwayMessageType
	unsubscribeMessageType
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
	case unannounceMessageType:
		return "AnannounceMessage"
	case goAwayMessageType:
		return "GoAwayMessage"
	case unsubscribeMessageType:
		return "UnsubscribeMessage"
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
	unannounceMessageMinimumLength       = 0
	goAwayMessageMinimumLength           = 0
	unsubscribeMessageMinimumLength      = 0
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
	case unannounceMessageType:
		return parseUnannounceMessage(reader, length)
	case goAwayMessageType:
		return parseGoAwayMessage(length)
	case unsubscribeMessageType:
		return parseUnsubscribeMessage(reader, length)
	}
	return nil, errors.New("unknown message type")
}

type objectMessage struct {
	trackID         uint64
	groupSequence   uint64
	objectSequence  uint64
	objectSendOrder uint64
	objectPayload   []byte
}

func (m objectMessage) String() string {
	return "ObjectMessage"
}

func (m *objectMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(objectMessageType))
	buf = varint.Append(buf,
		uint64(varint.Len(m.trackID))+
			uint64(varint.Len(m.groupSequence))+
			uint64(varint.Len(m.objectSequence))+
			uint64(varint.Len(m.objectSendOrder))+
			uint64(len(m.objectPayload)),
	)

	buf = varint.Append(buf, m.trackID)
	buf = varint.Append(buf, m.groupSequence)
	buf = varint.Append(buf, m.objectSequence)
	buf = varint.Append(buf, m.objectSendOrder)
	buf = append(buf, m.objectPayload...)
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
			trackID:         trackID,
			groupSequence:   groupSequence,
			objectSequence:  objectSequence,
			objectSendOrder: objectSendOrder,
			objectPayload:   objectPayload,
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
			trackID:         trackID,
			groupSequence:   groupSequence,
			objectSequence:  objectSequence,
			objectSendOrder: objectSendOrder,
			objectPayload:   objectPayload,
		}, err
	}
	return &objectMessage{
		trackID:         trackID,
		groupSequence:   groupSequence,
		objectSequence:  objectSequence,
		objectSendOrder: objectSendOrder,
		objectPayload:   []byte{},
	}, err
}

type clientSetupMessage struct {
	supportedVersions versions
	setupParameters   parameters
}

func (m clientSetupMessage) String() string {
	return "ClientSetupMessage"
}

func (m *clientSetupMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(setupMessageType))
	buf = varint.Append(buf,
		uint64(varint.Len(uint64(len(m.supportedVersions))))+
			m.supportedVersions.Len()+
			m.setupParameters.length(),
	)
	buf = varint.Append(buf, uint64(len(m.supportedVersions)))
	for _, v := range m.supportedVersions {
		buf = varint.Append(buf, uint64(v))
	}
	for _, p := range m.setupParameters {
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
		supportedVersions: versions,
		setupParameters:   ps,
	}, nil
}

type serverSetupMessage struct {
	selectedVersion version
	setupParameters parameters
}

func (m serverSetupMessage) String() string {
	return "ServerSetupMessage"
}

func (m *serverSetupMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(setupMessageType))
	buf = varint.Append(buf,
		m.selectedVersion.Len()+
			m.setupParameters.length(),
	)
	buf = varint.Append(buf, uint64(m.selectedVersion))
	for _, p := range m.setupParameters {
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
		selectedVersion: version(sv),
		setupParameters: ps,
	}, nil
}

type subscribeRequestMessage struct {
	fullTrackName          string
	trackRequestParameters parameters
}

func (m subscribeRequestMessage) String() string {
	out := "SubscribeRequestMessage"
	out += fmt.Sprintf("\tFullTrackName: %v\n", m.fullTrackName)
	out += fmt.Sprintf("\tTrackRequestParameters: %v\n", m.trackRequestParameters)
	return out
}

func (m subscribeRequestMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: m.fullTrackName,
	}
}

func (m *subscribeRequestMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeRequestMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.fullTrackName)+
			m.trackRequestParameters.length(),
	)
	buf = appendVarIntString(buf, m.fullTrackName)
	for _, p := range m.trackRequestParameters {
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
		fullTrackName:          name,
		trackRequestParameters: ps,
	}, nil
}

type subscribeOkMessage struct {
	fullTrackName string
	trackID       uint64
	expires       time.Duration
}

func (m subscribeOkMessage) String() string {
	out := "SubscribeOkMessage"
	out += fmt.Sprintf("\tFullTrackName: %v\n", m.fullTrackName)
	out += fmt.Sprintf("\tTrackID: %v\n", m.trackID)
	out += fmt.Sprintf("\tExpires: %v\n", m.expires)
	return out
}

func (m subscribeOkMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: m.fullTrackName,
	}
}

func (m *subscribeOkMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeOkMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.fullTrackName)+
			uint64(varint.Len(m.trackID))+
			uint64(varint.Len(uint64(m.expires))),
	)
	buf = appendVarIntString(buf, m.fullTrackName)
	buf = varint.Append(buf, m.trackID)
	buf = varint.Append(buf, uint64(m.expires))
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
		fullTrackName: fullTrackName,
		trackID:       trackID,
		expires:       time.Duration(e) * time.Millisecond,
	}, nil
}

type subscribeErrorMessage struct {
	fullTrackName string
	errorCode     uint64
	reasonPhrase  string
}

func (m subscribeErrorMessage) String() string {
	out := "SubscribeErrorMessage"
	out += fmt.Sprintf("\tFullTrackName: %v\n", m.fullTrackName)
	out += fmt.Sprintf("\tTrackID: %v\n", m.errorCode)
	out += fmt.Sprintf("\tExpires: %v\n", m.reasonPhrase)
	return out
}

func (m subscribeErrorMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: m.fullTrackName,
	}
}

func (m *subscribeErrorMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeErrorMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.fullTrackName)+
			uint64(varint.Len(m.errorCode))+
			varIntStringLen(m.reasonPhrase),
	)
	buf = appendVarIntString(buf, m.fullTrackName)
	buf = varint.Append(buf, m.errorCode)
	buf = appendVarIntString(buf, m.reasonPhrase)
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
		fullTrackName: fullTrackName,
		errorCode:     errorCode,
		reasonPhrase:  reasonPhrase,
	}, err
}

// TODO: This is technicall almost identical to AnnounceOkMessage so it should
// probably share some code?
type unsubscribeMessage struct {
	trackNamespace string
}

func (m unsubscribeMessage) String() string {
	out := "UnsubscribeMessage"
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	return out
}

func (m *unsubscribeMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(unsubscribeMessageType))
	buf = varint.Append(buf, uint64(len(m.trackNamespace)))
	buf = append(buf, []byte(m.trackNamespace)...)
	return buf
}

func parseUnsubscribeMessage(r messageReader, length int) (*unsubscribeMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length > 0 {
		buf := make([]byte, length)
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return nil, err
		}
		return &unsubscribeMessage{
			trackNamespace: string(buf),
		}, err
	}
	name, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &unsubscribeMessage{
		trackNamespace: string(name),
	}, err
}

type announceMessage struct {
	trackNamespace         string
	trackRequestParameters parameters
}

func (m announceMessage) String() string {
	out := "AnnounceMessage"
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	out += fmt.Sprintf("\tTrackRequestParameters: %v\n", m.trackRequestParameters)
	return out
}

func (m announceMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.trackNamespace,
	}
}

func (m *announceMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(announceMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.trackNamespace)+
			m.trackRequestParameters.length(),
	)
	buf = appendVarIntString(buf, m.trackNamespace)
	for _, p := range m.trackRequestParameters {
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
		trackNamespace:         trackNamspace,
		trackRequestParameters: ps,
	}, nil
}

type announceOkMessage struct {
	trackNamespace string
}

func (m announceOkMessage) String() string {
	out := "AnnounceOkMessage"
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	return out
}

func (m announceOkMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.trackNamespace,
	}
}

func (m *announceOkMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(announceOkMessageType))
	buf = varint.Append(buf, uint64(len(m.trackNamespace)))
	buf = append(buf, []byte(m.trackNamespace)...)
	return buf
}

func parseAnnounceOkMessage(r messageReader, length int) (*announceOkMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length > 0 {
		buf := make([]byte, length)
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return nil, err
		}
		return &announceOkMessage{
			trackNamespace: string(buf),
		}, err
	}
	name, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &announceOkMessage{
		trackNamespace: string(name),
	}, err
}

type announceErrorMessage struct {
	trackNamespace string
	errorCode      uint64
	reasonPhrase   string
}

func (m announceErrorMessage) String() string {
	out := "AnnounceErrorMessage"
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	out += fmt.Sprintf("\tErrorCdoe: %v\n", m.errorCode)
	out += fmt.Sprintf("\tReasonPhrase: %v\n", m.reasonPhrase)
	return out
}

func (m announceErrorMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.trackNamespace,
	}
}

func (m *announceErrorMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(announceErrorMessageType))
	buf = varint.Append(buf,
		varIntStringLen(m.trackNamespace)+
			uint64(varint.Len(m.errorCode))+
			varIntStringLen(m.reasonPhrase),
	)
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = varint.Append(buf, m.errorCode)
	buf = appendVarIntString(buf, m.reasonPhrase)
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
		trackNamespace: trackNamspace,
		errorCode:      errorCode,
		reasonPhrase:   reasonPhrase,
	}, err
}

type unannounceMessage struct {
	trackNamespace string
}

// TODO: This is technicall almost identical to AnnounceOkMessage so it should
// probably share some code?
func (m unannounceMessage) String() string {
	out := "UnannounceMessage"
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	return out
}

func (m *unannounceMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(unannounceMessageType))
	buf = varint.Append(buf, uint64(len(m.trackNamespace)))
	buf = append(buf, []byte(m.trackNamespace)...)
	return buf
}

func parseUnannounceMessage(r messageReader, length int) (*unannounceMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if length > 0 {
		buf := make([]byte, length)
		_, err := io.ReadFull(r, buf)
		if err != nil {
			return nil, err
		}
		return &unannounceMessage{
			trackNamespace: string(buf),
		}, err
	}
	name, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &unannounceMessage{
		trackNamespace: string(name),
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
