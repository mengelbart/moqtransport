package moqtransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errInvalidMessageReader   = errors.New("invalid message reader")
	errInvalidMessageEncoding = errors.New("invalid message encoding")
)

const (
	subscribeLocationModeNone = iota
	subscribeLocationModeAbsolute
	subscribeLocationModeRelativePrevious
	subscribeLocationModeRelativeNext
)

type role int

const (
	serverRole role = iota
	clientRole
)

type message interface {
	append([]byte) []byte
}

type messageType uint64

const (
	objectMessageLenType messageType = iota

	objectMessageNoLenType messageType = iota + 1
	subscribeRequestMessageType
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

	clientSetupMessageType messageType = 0x40
	serverSetupMessageType messageType = 0x41
)

func (mt messageType) String() string {
	switch mt {
	case objectMessageLenType:
		return "ObjectLenMessage"
	case objectMessageNoLenType:
		return "ObjectNoLenMessage"
	case subscribeRequestMessageType:
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
	case clientSetupMessageType:
		return "ClientSetupMessage"
	case serverSetupMessageType:
		return "ServerSetupMessage"
	}
	return "unknown message type"
}

type messageReader interface {
	io.Reader
	io.ByteReader
}

func readNext(reader messageReader) (msg message, err error) {
	mt, err := quicvarint.Read(reader)
	if err != nil {
		return nil, err
	}
	switch messageType(mt) {
	case objectMessageLenType:
		msg, err = parseObjectMessage(reader, mt)
	case objectMessageNoLenType:
		msg, err = parseObjectMessage(reader, mt)
	case subscribeRequestMessageType:
		msg, err = parseSubscribeRequestMessage(reader)
	case subscribeOkMessageType:
		msg, err = parseSubscribeOkMessage(reader)
	case subscribeErrorMessageType:
		msg, err = parseSubscribeErrorMessage(reader)
	case announceMessageType:
		msg, err = parseAnnounceMessage(reader)
	case announceOkMessageType:
		msg, err = parseAnnounceOkMessage(reader)
	case announceErrorMessageType:
		msg, err = parseAnnounceErrorMessage(reader)
	case unannounceMessageType:
		msg, err = parseUnannounceMessage(reader)
	case unsubscribeMessageType:
		msg, err = parseUnsubscribeMessage(reader)
	case subscribeFinMessageType:
		msg, err = parseSubscribeFinMessage(reader)
	case subscribeRstMessageType:
		msg, err = parseSubscribeRstMessage(reader)
	case goAwayMessageType:
		msg, err = parseGoAwayMessage(reader)
	case clientSetupMessageType:
		msg, err = parseClientSetupMessage(reader)
	case serverSetupMessageType:
		msg, err = parseServerSetupMessage(reader)
	default:
		return nil, errors.New("unknown message type")
	}
	log.Printf("parsed message: %v, err: %v", msg, err)
	return
}

type objectMessage struct {
	HasLength bool

	TrackID         uint64
	GroupSequence   uint64
	ObjectSequence  uint64
	ObjectSendOrder uint64
	ObjectPayload   []byte
}

func (m objectMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	if m.HasLength {
		return fmt.Sprintf("%v:%v", objectMessageLenType, string(buf))
	}
	return fmt.Sprintf("%v:%v", objectMessageNoLenType, string(buf))
}

func (m *objectMessage) append(buf []byte) []byte {
	if m.HasLength {
		buf = quicvarint.Append(buf, uint64(objectMessageLenType))
	} else {
		buf = quicvarint.Append(buf, uint64(objectMessageNoLenType))
	}
	buf = quicvarint.Append(buf, m.TrackID)
	buf = quicvarint.Append(buf, m.GroupSequence)
	buf = quicvarint.Append(buf, m.ObjectSequence)
	buf = quicvarint.Append(buf, m.ObjectSendOrder)
	if m.HasLength {
		buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	}
	buf = append(buf, m.ObjectPayload...)
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
	trackID, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	groupSequence, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	objectSequence, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	objectSendOrder, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	if !hasLen {
		var objectPayload []byte
		objectPayload, err = io.ReadAll(r)
		return &objectMessage{
			HasLength:       hasLen,
			TrackID:         trackID,
			GroupSequence:   groupSequence,
			ObjectSequence:  objectSequence,
			ObjectSendOrder: objectSendOrder,
			ObjectPayload:   objectPayload,
		}, err
	}
	length, err := quicvarint.Read(r)
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
			HasLength:       hasLen,
			TrackID:         trackID,
			GroupSequence:   groupSequence,
			ObjectSequence:  objectSequence,
			ObjectSendOrder: objectSendOrder,
			ObjectPayload:   objectPayload,
		}, err
	}
	return &objectMessage{
		HasLength:       hasLen,
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
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", clientSetupMessageType.String(), string(buf))
}

func (m *clientSetupMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(clientSetupMessageType))
	buf = quicvarint.Append(buf, uint64(len(m.SupportedVersions)))
	for _, v := range m.SupportedVersions {
		buf = quicvarint.Append(buf, uint64(v))
	}
	buf = quicvarint.Append(buf, uint64(len(m.SetupParameters)))
	for _, p := range m.SetupParameters {
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
		SupportedVersions: vs,
		SetupParameters:   ps,
	}, nil
}

type serverSetupMessage struct {
	SelectedVersion version
	SetupParameters parameters
}

func (m serverSetupMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", serverSetupMessageType.String(), string(buf))
}

func (m *serverSetupMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(serverSetupMessageType))
	buf = quicvarint.Append(buf, uint64(m.SelectedVersion))
	buf = quicvarint.Append(buf, uint64(len(m.SetupParameters)))
	for _, p := range m.SetupParameters {
		buf = p.append(buf)
	}
	return buf
}

func parseServerSetupMessage(r messageReader) (*serverSetupMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	sv, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r)
	if err != nil {
		return nil, err
	}
	return &serverSetupMessage{
		SelectedVersion: version(sv),
		SetupParameters: ps,
	}, nil
}

type location struct {
	mode  uint64
	value uint64
}

func (l location) append(buf []byte) []byte {
	if l.mode == 0 {
		return append(buf, byte(l.mode))
	}
	buf = append(buf, byte(l.mode))
	return append(buf, byte(l.value))
}

func parseLocation(r messageReader) (location, error) {
	if r == nil {
		return location{}, errInvalidMessageReader
	}
	mode, err := quicvarint.Read(r)
	if err != nil {
		return location{}, err
	}
	if mode == subscribeLocationModeNone {
		return location{
			mode:  mode,
			value: 0,
		}, nil
	}
	value, err := quicvarint.Read(r)
	if err != nil {
		return location{}, nil
	}
	return location{
		mode:  mode,
		value: value,
	}, nil
}

type subscribeRequestMessage struct {
	TrackNamespace string
	TrackName      string
	StartGroup     location
	StartObject    location
	EndGroup       location
	EndObject      location
	Parameters     parameters
}

func (m subscribeRequestMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", subscribeRequestMessageType.String(), string(buf))
}

func (m subscribeRequestMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: fmt.Sprintf("%v/%v", m.TrackNamespace, m.TrackName),
	}
}

func (m *subscribeRequestMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeRequestMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
	buf = m.StartGroup.append(buf)
	buf = m.StartObject.append(buf)
	buf = m.EndGroup.append(buf)
	buf = m.EndObject.append(buf)
	buf = quicvarint.Append(buf, uint64(len(m.Parameters)))
	for _, p := range m.Parameters {
		buf = p.append(buf)
	}
	return buf
}

func parseSubscribeRequestMessage(r messageReader) (*subscribeRequestMessage, error) {
	if r == nil {
		return nil, errInvalidMessageReader
	}
	trackNamespace, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	fullTrackName, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	startGroup, err := parseLocation(r)
	if err != nil {
		return nil, err
	}
	startObject, err := parseLocation(r)
	if err != nil {
		return nil, err
	}
	endGroup, err := parseLocation(r)
	if err != nil {
		return nil, err
	}
	endObject, err := parseLocation(r)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(r)
	if err != nil {
		return nil, err
	}
	return &subscribeRequestMessage{
		TrackNamespace: trackNamespace,
		TrackName:      fullTrackName,
		StartGroup:     startGroup,
		StartObject:    startObject,
		EndGroup:       endGroup,
		EndObject:      endObject,
		Parameters:     ps,
	}, nil
}

type subscribeOkMessage struct {
	TrackNamespace string
	TrackName      string
	TrackID        uint64
	Expires        time.Duration
}

func (m subscribeOkMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", subscribeOkMessageType.String(), string(buf))
}

func (m subscribeOkMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: fmt.Sprintf("%v/%v", m.TrackNamespace, m.TrackName),
	}
}

func (m *subscribeOkMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeOkMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
	buf = quicvarint.Append(buf, m.TrackID)
	buf = quicvarint.Append(buf, uint64(m.Expires))
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
	trackID, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	e, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	return &subscribeOkMessage{
		TrackNamespace: namespace,
		TrackName:      trackName,
		TrackID:        trackID,
		Expires:        time.Duration(e) * time.Millisecond,
	}, nil
}

type subscribeErrorMessage struct {
	TrackNamespace string
	TrackName      string
	ErrorCode      uint64
	ReasonPhrase   string
}

func (m subscribeErrorMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", subscribeErrorMessageType.String(), string(buf))
}

func (m subscribeErrorMessage) key() messageKey {
	return messageKey{
		mt: subscribeRequestMessageType,
		id: fmt.Sprintf("%v/%v", m.TrackNamespace, m.TrackName),
	}
}

func (m *subscribeErrorMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeErrorMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
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
	trackName, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	errorCode, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(r)
	return &subscribeErrorMessage{
		TrackNamespace: namespace,
		TrackName:      trackName,
		ErrorCode:      errorCode,
		ReasonPhrase:   reasonPhrase,
	}, err
}

type unsubscribeMessage struct {
	TrackNamespace string
	TrackName      string
}

func (m unsubscribeMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", unsubscribeMessageType.String(), string(buf))
}

func (m *unsubscribeMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(unsubscribeMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
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
		TrackNamespace: namespace,
		TrackName:      trackName,
	}, err
}

type subscribeFinMessage struct {
	TrackNamespace string
	TrackName      string
	FinalGroup     uint64
	FinalObject    uint64
}

func (m subscribeFinMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", subscribeFinMessageType.String(), string(buf))
}

func (m *subscribeFinMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeFinMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
	buf = quicvarint.Append(buf, m.FinalGroup)
	buf = quicvarint.Append(buf, m.FinalObject)
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
	finalGroup, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	finalObject, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	return &subscribeFinMessage{
		TrackNamespace: namespace,
		TrackName:      name,
		FinalGroup:     finalGroup,
		FinalObject:    finalObject,
	}, nil
}

type subscribeRstMessage struct {
	TrackNamespace string
	TrackName      string
	ErrorCode      uint64
	ReasonPhrase   string
	FinalGroup     uint64
	FinalObject    uint64
}

func (m subscribeRstMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", subscribeRstMessageType.String(), string(buf))
}

func (m *subscribeRstMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeRstMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = appendVarIntString(buf, m.TrackName)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
	buf = quicvarint.Append(buf, m.FinalGroup)
	buf = quicvarint.Append(buf, m.FinalObject)
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
	errCode, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	finalGroup, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	finalObject, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	return &subscribeRstMessage{
		TrackNamespace: namespace,
		TrackName:      name,
		ErrorCode:      errCode,
		ReasonPhrase:   reasonPhrase,
		FinalGroup:     finalGroup,
		FinalObject:    finalObject,
	}, nil
}

type announceMessage struct {
	TrackNamespace         string
	TrackRequestParameters parameters
}

func (m announceMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", announceMessageType.String(), string(buf))
}

func (m announceMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *announceMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = quicvarint.Append(buf, uint64(len(m.TrackRequestParameters)))
	for _, p := range m.TrackRequestParameters {
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
		TrackNamespace:         trackNamspace,
		TrackRequestParameters: ps,
	}, nil
}

type announceOkMessage struct {
	TrackNamespace string
}

func (m announceOkMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", announceOkMessageType.String(), string(buf))
}

func (m announceOkMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *announceOkMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceOkMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
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
		TrackNamespace: namespace,
	}, err
}

type announceErrorMessage struct {
	TrackNamespace string
	ErrorCode      uint64
	ReasonPhrase   string
}

func (m announceErrorMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", announceErrorMessageType.String(), string(buf))
}

func (m announceErrorMessage) key() messageKey {
	return messageKey{
		mt: announceMessageType,
		id: m.TrackNamespace,
	}
}

func (m *announceErrorMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceErrorMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
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
	errorCode, err := quicvarint.Read(r)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(r)
	if err != nil {
		return nil, err
	}
	return &announceErrorMessage{
		TrackNamespace: trackNamspace,
		ErrorCode:      errorCode,
		ReasonPhrase:   reasonPhrase,
	}, nil
}

type unannounceMessage struct {
	TrackNamespace string
}

func (m unannounceMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", unannounceMessageType.String(), string(buf))
}

func (m *unannounceMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(unannounceMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
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
		TrackNamespace: namespace,
	}, nil
}

type goAwayMessage struct {
	NewSessionURI string
}

func (m goAwayMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%v:%v", goAwayMessageType.String(), string(buf))
}

func (m *goAwayMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(goAwayMessageType))
	buf = appendVarIntString(buf, m.NewSessionURI)
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
		NewSessionURI: uri,
	}, nil
}
