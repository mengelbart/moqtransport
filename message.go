package moqtransport

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mengelbart/moqtransport/varint"
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
	objectMessageLenType messageType = iota
	setupMessageType
	objectMessageNoLenType
	subscribeMessageType
	subscribeOkMessageType
	subscribeErrorMessageType
	announceMessageType
	announceOkMessageType
	announceErrorMessageType
	unannounceMessageType
	unsubscribeMessageType
	subscribeFinMessageType
	subscribeRstMessageType

	goAwayMessageType messageType = 0x10
)

func (mt messageType) String() string {
	switch mt {
	case objectMessageLenType:
		return "ObjectLenMessage"
	case setupMessageType:
		return "SetupMessage"
	case objectMessageNoLenType:
		return "ObjectNoLenMessage"
	case subscribeMessageType:
		return "SubscribeMessage"
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
	case unsubscribeMessageType:
		return "UnsubscribeMessage"
	case subscribeFinMessageType:
		return "SubscribeFinMessage"
	case subscribeRstMessageType:
		return "SubscribeRstMessage"
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
	switch messageType(mt) {
	case objectMessageLenType:
		msg, err := parseObjectMessage(reader, mt)
		return msg, err
	case setupMessageType:
		switch r {
		case serverRole:
			csm, err := parseClientSetupMessage(reader)
			return csm, err
		case clientRole:
			ssm, err := parseServerSetupMessage(reader)
			return ssm, err
		}
	case subscribeMessageType:
		srm, err := parseSubscribeMessage(reader)
		return srm, err
	case subscribeOkMessageType:
		som, err := parseSubscribeOkMessage(reader)
		return som, err
	case subscribeErrorMessageType:
		sem, err := parseSubscribeErrorMessage(reader)
		return sem, err
	case announceMessageType:
		am, err := parseAnnounceMessage(reader)
		return am, err
	case announceOkMessageType:
		aom, err := parseAnnounceOkMessage(reader)
		return aom, err
	case announceErrorMessageType:
		return parseAnnounceErrorMessage(reader)
	case unannounceMessageType:
		return parseUnannounceMessage(reader)
	case goAwayMessageType:
		return parseGoAwayMessage(reader)
	case unsubscribeMessageType:
		return parseUnsubscribeMessage(reader)
	}
	return nil, errors.New("unknown message type")
}

type objectMessage struct {
	hasLength bool

	trackID         uint64
	groupSequence   uint64
	objectSequence  uint64
	objectSendOrder uint64
	objectPayload   []byte
}

func (m objectMessage) String() string {
	if m.hasLength {
		return objectMessageLenType.String()
	}
	return objectMessageNoLenType.String()
}

func (m *objectMessage) append(buf []byte) []byte {
	if m.hasLength {
		buf = varint.Append(buf, uint64(objectMessageLenType))
	} else {
		buf = varint.Append(buf, uint64(objectMessageNoLenType))
	}
	buf = varint.Append(buf, m.trackID)
	buf = varint.Append(buf, m.groupSequence)
	buf = varint.Append(buf, m.objectSequence)
	buf = varint.Append(buf, m.objectSendOrder)
	if m.hasLength {
		buf = varint.Append(buf, uint64(len(m.objectPayload)))
	}
	buf = append(buf, m.objectPayload...)
	return buf
}

func parseObjectMessage(r messageReader, typ uint64) (*objectMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	if typ != uint64(objectMessageLenType) && typ != uint64(objectMessageNoLenType) {
		return nil, errInvalidMessageEncoding
	}
	hasLen := typ == 0x00
	trackID, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	groupSequence, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	objectSequence, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	objectSendOrder, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	if !hasLen {
		var objectPayload []byte
		objectPayload, err = io.ReadAll(r)
		return &objectMessage{
			hasLength:       hasLen,
			trackID:         trackID,
			groupSequence:   groupSequence,
			objectSequence:  objectSequence,
			objectSendOrder: objectSendOrder,
			objectPayload:   objectPayload,
		}, err
	}
	length, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	if length > 0 {
		objectPayload := make([]byte, length)
		_, err = io.ReadFull(r, objectPayload)
		if err != nil {
			return nil, err
		}
		return &objectMessage{
			hasLength:       hasLen,
			trackID:         trackID,
			groupSequence:   groupSequence,
			objectSequence:  objectSequence,
			objectSendOrder: objectSendOrder,
			objectPayload:   objectPayload,
		}, err
	}
	return &objectMessage{
		hasLength:       hasLen,
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
	return setupMessageType.String()
}

func (m *clientSetupMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(setupMessageType))
	buf = varint.Append(buf, uint64(len(m.supportedVersions)))
	for _, v := range m.supportedVersions {
		buf = varint.Append(buf, uint64(v))
	}
	buf = varint.Append(buf, uint64(len(m.setupParameters)))
	for _, p := range m.setupParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseClientSetupMessage(r messageReader) (*clientSetupMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	vs, err := parseVersions(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r)
	if err != nil {
		return nil, err
	}
	return &clientSetupMessage{
		supportedVersions: vs,
		setupParameters:   ps,
	}, nil
}

type serverSetupMessage struct {
	selectedVersion version
	setupParameters parameters
}

func (m serverSetupMessage) String() string {
	return setupMessageType.String()
}

func (m *serverSetupMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(setupMessageType))
	buf = varint.Append(buf, uint64(m.selectedVersion))
	buf = varint.Append(buf, uint64(len(m.setupParameters)))
	for _, p := range m.setupParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseServerSetupMessage(r messageReader) (*serverSetupMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	sv, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r)
	if err != nil {
		return nil, err
	}
	return &serverSetupMessage{
		selectedVersion: version(sv),
		setupParameters: ps,
	}, nil
}

type subscribeMessage struct {
	trackNamespace string
	trackName      string
	parameters     parameters
}

func (m subscribeMessage) String() string {
	out := subscribeMessageType.String()
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	out += fmt.Sprintf("\tTrackName: %v\n", m.trackName)
	out += fmt.Sprintf("\tTrackRequestParameters: %v\n", m.parameters)
	return out
}

func (m subscribeMessage) key() messageKey {
	return messageKey{
		mt: subscribeMessageType,
		id: m.trackName,
	}
}

func (m *subscribeMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeMessageType))
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = appendVarIntString(buf, m.trackName)
	buf = varint.Append(buf, uint64(len(m.parameters)))
	for _, p := range m.parameters {
		buf = p.append(buf)
	}
	return buf
}

func parseSubscribeMessage(r messageReader) (*subscribeMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	name, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r)
	if err != nil {
		return nil, err
	}
	return &subscribeMessage{
		trackNamespace: namespace,
		trackName:      name,
		parameters:     ps,
	}, nil
}

type subscribeOkMessage struct {
	trackNamespace string
	trackName      string
	trackID        uint64
	expires        time.Duration
}

func (m subscribeOkMessage) String() string {
	out := subscribeOkMessageType.String()
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	out += fmt.Sprintf("\tFullTrackName: %v\n", m.trackName)
	out += fmt.Sprintf("\tTrackID: %v\n", m.trackID)
	out += fmt.Sprintf("\tExpires: %v\n", m.expires)
	return out
}

func (m subscribeOkMessage) key() messageKey {
	return messageKey{
		mt: subscribeMessageType,
		id: m.trackName,
	}
}

func (m *subscribeOkMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeOkMessageType))
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = appendVarIntString(buf, m.trackName)
	buf = varint.Append(buf, m.trackID)
	buf = varint.Append(buf, uint64(m.expires))
	return buf
}

func parseSubscribeOkMessage(r messageReader) (*subscribeOkMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	trackName, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	trackID, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	e, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	return &subscribeOkMessage{
		trackNamespace: namespace,
		trackName:      trackName,
		trackID:        trackID,
		expires:        time.Duration(e) * time.Millisecond,
	}, nil
}

type subscribeErrorMessage struct {
	trackNamespace string
	trackName      string
	errorCode      uint64
	reasonPhrase   string
}

func (m subscribeErrorMessage) String() string {
	out := subscribeErrorMessageType.String()
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	out += fmt.Sprintf("\tTrackName: %v\n", m.trackName)
	out += fmt.Sprintf("\tTrackID: %v\n", m.errorCode)
	out += fmt.Sprintf("\tExpires: %v\n", m.reasonPhrase)
	return out
}

func (m subscribeErrorMessage) key() messageKey {
	return messageKey{
		mt: subscribeMessageType,
		id: m.trackName,
	}
}

func (m *subscribeErrorMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeErrorMessageType))
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = appendVarIntString(buf, m.trackName)
	buf = varint.Append(buf, m.errorCode)
	buf = appendVarIntString(buf, m.reasonPhrase)
	return buf
}

func parseSubscribeErrorMessage(r messageReader) (*subscribeErrorMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	fullTrackName, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	errorCode, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(r)
	return &subscribeErrorMessage{
		trackNamespace: namespace,
		trackName:      fullTrackName,
		errorCode:      errorCode,
		reasonPhrase:   reasonPhrase,
	}, err
}

type unsubscribeMessage struct {
	trackNamespace string
	trackName      string
}

func (m unsubscribeMessage) String() string {
	out := unsubscribeMessageType.String()
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	out += fmt.Sprintf("\tTrackName: %v\n", m.trackName)
	return out
}

func (m *unsubscribeMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(unsubscribeMessageType))
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = appendVarIntString(buf, m.trackName)
	return buf
}

func parseUnsubscribeMessage(r messageReader) (*unsubscribeMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	trackName, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	return &unsubscribeMessage{
		trackNamespace: namespace,
		trackName:      trackName,
	}, err
}

type subscribeFinMessage struct {
	trackNamespace string
	trackName      string
	finalGroup     uint64
	finalObject    uint64
}

func (m *subscribeFinMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeFinMessageType))
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = appendVarIntString(buf, m.trackName)
	buf = varint.Append(buf, m.finalGroup)
	buf = varint.Append(buf, m.finalObject)
	return buf
}

func parseSubscribeFinMessage(r messageReader) (*subscribeFinMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	name, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	finalGroup, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	finalObject, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	return &subscribeFinMessage{
		trackNamespace: namespace,
		trackName:      name,
		finalGroup:     finalGroup,
		finalObject:    finalObject,
	}, nil
}

type subscribeRstMessage struct {
	trackNamespace string
	trackName      string
	errorCode      uint64
	reasonPhrase   string
	finalGroup     uint64
	finalObject    uint64
}

func (m *subscribeRstMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(subscribeRstMessageType))
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = appendVarIntString(buf, m.trackName)
	buf = varint.Append(buf, m.errorCode)
	buf = appendVarIntString(buf, m.reasonPhrase)
	buf = varint.Append(buf, m.finalGroup)
	buf = varint.Append(buf, m.finalObject)
	return buf
}

func parseSubscribeRstMessage(r messageReader) (*subscribeRstMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	name, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	errCode, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	finalGroup, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	finalObject, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	return &subscribeRstMessage{
		trackNamespace: namespace,
		trackName:      name,
		errorCode:      errCode,
		reasonPhrase:   reasonPhrase,
		finalGroup:     finalGroup,
		finalObject:    finalObject,
	}, nil
}

type announceMessage struct {
	trackNamespace         string
	trackRequestParameters parameters
}

func (m announceMessage) String() string {
	out := announceMessageType.String()
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
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = varint.Append(buf, uint64(len(m.trackRequestParameters)))
	for _, p := range m.trackRequestParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseAnnounceMessage(r messageReader) (*announceMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	trackNamspace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r)
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
	out := announceOkMessageType.String()
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
	buf = appendVarIntString(buf, m.trackNamespace)
	return buf
}

func parseAnnounceOkMessage(r messageReader) (*announceOkMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	return &announceOkMessage{
		trackNamespace: namespace,
	}, err
}

type announceErrorMessage struct {
	trackNamespace string
	errorCode      uint64
	reasonPhrase   string
}

func (m announceErrorMessage) String() string {
	out := announceErrorMessageType.String()
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
	buf = appendVarIntString(buf, m.trackNamespace)
	buf = varint.Append(buf, m.errorCode)
	buf = appendVarIntString(buf, m.reasonPhrase)
	return buf
}

func parseAnnounceErrorMessage(r messageReader) (*announceErrorMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	trackNamspace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	errorCode, err := varint.Read(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	return &announceErrorMessage{
		trackNamespace: trackNamspace,
		errorCode:      errorCode,
		reasonPhrase:   reasonPhrase,
	}, nil
}

type unannounceMessage struct {
	trackNamespace string
}

func (m unannounceMessage) String() string {
	out := unannounceMessageType.String()
	out += fmt.Sprintf("\tTrackNamespace: %v\n", m.trackNamespace)
	return out
}

func (m *unannounceMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(unannounceMessageType))
	buf = appendVarIntString(buf, m.trackNamespace)
	return buf
}

func parseUnannounceMessage(r messageReader) (*unannounceMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	return &unannounceMessage{
		trackNamespace: namespace,
	}, nil
}

type goAwayMessage struct {
	newSessionURI string
}

func (m goAwayMessage) String() string {
	out := goAwayMessageType.String()
	out += fmt.Sprintf("\tNewSessionURI: %v\n", m.newSessionURI)
	return out
}

func (m *goAwayMessage) append(buf []byte) []byte {
	buf = varint.Append(buf, uint64(goAwayMessageType))
	buf = appendVarIntString(buf, m.newSessionURI)
	return buf
}

func parseGoAwayMessage(r messageReader) (*goAwayMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	uri, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	return &goAwayMessage{
		newSessionURI: uri,
	}, nil
}
