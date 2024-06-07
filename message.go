package moqtransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	ErrorCodeNoError                 = 0x00
	ErrorCodeInternal                = 0x01
	ErrorCodeUnauthorized            = 0x02
	ErrorCodeProtocolViolation       = 0x03
	ErrorCodeDuplicateTrackAlias     = 0x04
	ErrorCodeParameterLengthMismatch = 0x05
	ErrorCodeGoAwayTimeout           = 0x10

	// Errors not included in current draft
	ErrorCodeUnsupportedVersion = 0xff01
	ErrorCodeTrackNotFound      = 0xff02
)

const (
	SubscribeStatusUnsubscribed      = 0x00
	SubscribeStatusInternalError     = 0x01
	SubscribeStatusUnauthorized      = 0x02
	SubscribeStatusTrackEnded        = 0x03
	SubscribeStatusSubscriptionEnded = 0x04
	SubscribeStatusGoingAway         = 0x05
	SubscribeStatusExpired           = 0x06
)

const (
	SubscribeErrorInternal        = 0x00
	SubscribeErrorInvalidRange    = 0x01
	SubscribeErrorRetryTrackAlias = 0x02

	// TODO: These are not specified yet, but seem useful
	SubscribeErrorUnknownTrack = 0x03
)

var (
	errInvalidMessageReader = errors.New("invalid message reader")
	errUnknownMessage       = errors.New("unknown message type")
)

type message interface {
	append([]byte) []byte
}

type messageType uint64

const (
	objectStreamMessageType      messageType = 0x00
	objectDatagramMessageType    messageType = 0x01
	subscribeMessageType         messageType = 0x03
	subscribeOkMessageType       messageType = 0x04
	subscribeErrorMessageType    messageType = 0x05
	announceMessageType          messageType = 0x06
	announceOkMessageType        messageType = 0x07
	announceErrorMessageType     messageType = 0x08
	unannounceMessageType        messageType = 0x09
	unsubscribeMessageType       messageType = 0x0a
	subscribeDoneMessageType     messageType = 0x0b
	announceCancelMessageType    messageType = 0x0c
	goAwayMessageType            messageType = 0x10
	clientSetupMessageType       messageType = 0x40
	serverSetupMessageType       messageType = 0x41
	streamHeaderTrackMessageType messageType = 0x50
	streamHeaderGroupMessageType messageType = 0x51
)

func (mt messageType) String() string {
	switch mt {
	case objectStreamMessageType:
		return "ObjectStreamMessage"
	case objectDatagramMessageType:
		return "objectDatagram"
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
	case subscribeDoneMessageType:
		return "SubscribeDoneMessage"
	case goAwayMessageType:
		return "GoAwayMessage"
	case clientSetupMessageType:
		return "ClientSetupMessage"
	case serverSetupMessageType:
		return "ServerSetupMessage"
	case streamHeaderTrackMessageType:
		return "StreamHeaderTrackMessage"
	case streamHeaderGroupMessageType:
		return "streamHeaderGroupMessage"
	case announceCancelMessageType:
		return "announceCancelMessageType"
	}
	return "unknown message type"
}

type messageReader interface {
	io.Reader
	io.ByteReader
}

type loggingParser struct {
	logger *slog.Logger
	reader messageReader
}

func newParser(mr messageReader) *loggingParser {
	return &loggingParser{
		logger: defaultLogger.WithGroup("MOQ_PARSER"),
		reader: mr,
	}
}

func (p *loggingParser) parse() (msg message, err error) {
	mt, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	switch messageType(mt) {
	case objectStreamMessageType, objectDatagramMessageType:
		msg, err = p.parseObjectMessage(messageType(mt))
	case subscribeMessageType:
		msg, err = p.parseSubscribeMessage()
	case subscribeOkMessageType:
		msg, err = p.parseSubscribeOkMessage()
	case subscribeErrorMessageType:
		msg, err = p.parseSubscribeErrorMessage()
	case announceMessageType:
		msg, err = p.parseAnnounceMessage()
	case announceOkMessageType:
		msg, err = p.parseAnnounceOkMessage()
	case announceErrorMessageType:
		msg, err = p.parseAnnounceErrorMessage()
	case unannounceMessageType:
		msg, err = p.parseUnannounceMessage()
	case unsubscribeMessageType:
		msg, err = p.parseUnsubscribeMessage()
	case subscribeDoneMessageType:
		msg, err = p.parseSubscribeDoneMessage()
	case goAwayMessageType:
		msg, err = p.parseGoAwayMessage()
	case clientSetupMessageType:
		msg, err = p.parseClientSetupMessage()
	case serverSetupMessageType:
		msg, err = p.parseServerSetupMessage()
	case streamHeaderTrackMessageType:
		msg, err = p.parseStreamHeaderTrackMessage()
	case streamHeaderGroupMessageType:
		msg, err = p.parseStreamHeaderGroupMessage()
	case announceCancelMessageType:
		msg, err = p.parseAnnounceCancelMessage()
	default:
		p.logger.Info("failed to parse message", "message_type", mt)
		return nil, errUnknownMessage
	}
	if err != nil {
		p.logger.Info("failed to parse message", "message_type", messageType(mt), "error", err)
	}
	return
}

type objectMessage struct {
	datagram        bool
	SubscribeID     uint64
	TrackAlias      uint64
	GroupID         uint64
	ObjectID        uint64
	ObjectSendOrder uint64
	ObjectPayload   []byte
}

func (m objectMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of objectMessage failed"
	}
	return fmt.Sprintf("%v:%v", objectStreamMessageType.String(), string(buf))
}

func (m *objectMessage) append(buf []byte) []byte {
	if m.datagram {
		buf = quicvarint.Append(buf, uint64(objectDatagramMessageType))
	} else {
		buf = quicvarint.Append(buf, uint64(objectStreamMessageType))
	}
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = quicvarint.Append(buf, m.ObjectSendOrder)
	buf = append(buf, m.ObjectPayload...)
	return buf
}

// TODO: Add prefer datagram property to object type?
func (p *loggingParser) parseObjectMessage(mt messageType) (*objectMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	trackAlias, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	groupID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	objectID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	objectSendOrder, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	// TODO: make the message type an io.Reader and let the user read
	objectPayload, err := io.ReadAll(p.reader)
	if err != nil {
		return nil, err
	}
	return &objectMessage{
		datagram:        mt == objectDatagramMessageType,
		SubscribeID:     subscribeID,
		TrackAlias:      trackAlias,
		GroupID:         groupID,
		ObjectID:        objectID,
		ObjectSendOrder: objectSendOrder,
		ObjectPayload:   objectPayload,
	}, nil
}

type clientSetupMessage struct {
	SupportedVersions versions
	SetupParameters   parameters
}

func (m clientSetupMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of clientSetupMessage failed"
	}
	return fmt.Sprintf("%v:%s", clientSetupMessageType.String(), string(buf))
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

func (p *loggingParser) parseClientSetupMessage() (*clientSetupMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	vs, err := parseVersions(p.reader)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(p.reader)
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
		return "json.Marshal of serverSetupMessage failed"
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

func (p *loggingParser) parseServerSetupMessage() (*serverSetupMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	sv, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(p.reader)
	if err != nil {
		return nil, err
	}
	return &serverSetupMessage{
		SelectedVersion: version(sv),
		SetupParameters: ps,
	}, nil
}

type subscribeMessage struct {
	SubscribeID    uint64
	TrackAlias     uint64
	TrackNamespace string
	TrackName      string
	StartGroup     Location
	StartObject    Location
	EndGroup       Location
	EndObject      Location
	Parameters     parameters
}

func (m subscribeMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of subscribeMessage failed"
	}
	return fmt.Sprintf("%v:%v", subscribeMessageType.String(), string(buf))
}

func (m subscribeMessage) subscribeID() uint64 {
	return m.SubscribeID
}

func (m *subscribeMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
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

func (p *loggingParser) parseSubscribeMessage() (*subscribeMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	trackAlias, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	trackNamespace, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	fullTrackName, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	startGroup, err := p.parseLocation()
	if err != nil {
		return nil, err
	}
	startObject, err := p.parseLocation()
	if err != nil {
		return nil, err
	}
	endGroup, err := p.parseLocation()
	if err != nil {
		return nil, err
	}
	endObject, err := p.parseLocation()
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(p.reader)
	if err != nil {
		return nil, err
	}
	return &subscribeMessage{
		SubscribeID:    subscribeID,
		TrackAlias:     trackAlias,
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
	SubscribeID   uint64
	Expires       time.Duration
	ContentExists bool
	FinalGroup    uint64
	FinalObject   uint64
}

func (m subscribeOkMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of subscribeOkMessage failed"
	}
	return fmt.Sprintf("%v:%v", subscribeOkMessageType.String(), string(buf))
}

func (m subscribeOkMessage) subscribeID() uint64 {
	return m.SubscribeID
}

func (m *subscribeOkMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeOkMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, uint64(m.Expires))
	if m.ContentExists {
		buf = append(buf, 1)
		buf = quicvarint.Append(buf, m.FinalGroup)
		buf = quicvarint.Append(buf, m.FinalObject)
		return buf
	}
	buf = append(buf, 0)
	return buf
}

func (p *loggingParser) parseSubscribeOkMessage() (*subscribeOkMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	expires, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	contentExistsByte, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	var contentExists bool
	switch contentExistsByte {
	case 0:
		contentExists = false
	case 1:
		contentExists = true
	default:
		return nil, errors.New("invalid use of ContentExists byte")
	}
	if !contentExists {
		return &subscribeOkMessage{
			SubscribeID:   subscribeID,
			Expires:       time.Duration(expires) * time.Millisecond,
			ContentExists: contentExists,
			FinalGroup:    0,
			FinalObject:   0,
		}, nil
	}
	finalGroup, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	finalObject, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	return &subscribeOkMessage{
		SubscribeID:   subscribeID,
		Expires:       time.Duration(expires) * time.Millisecond,
		ContentExists: contentExists,
		FinalGroup:    finalGroup,
		FinalObject:   finalObject,
	}, nil
}

type subscribeErrorMessage struct {
	SubscribeID  uint64
	ErrorCode    uint64
	ReasonPhrase string
	TrackAlias   uint64
}

func (m subscribeErrorMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of subscribeErrorMessage failed"
	}
	return fmt.Sprintf("%v:%v", subscribeErrorMessageType.String(), string(buf))
}

func (m subscribeErrorMessage) subscribeID() uint64 {
	return m.SubscribeID
}

func (m *subscribeErrorMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeErrorMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, uint64(m.ErrorCode))
	buf = appendVarIntString(buf, m.ReasonPhrase)
	buf = quicvarint.Append(buf, m.TrackAlias)
	return buf
}

func (p *loggingParser) parseSubscribeErrorMessage() (*subscribeErrorMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	errorCode, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	trackAlias, err := quicvarint.Read(p.reader)
	return &subscribeErrorMessage{
		SubscribeID:  subscribeID,
		ErrorCode:    errorCode,
		ReasonPhrase: reasonPhrase,
		TrackAlias:   trackAlias,
	}, err
}

type unsubscribeMessage struct {
	SubscribeID uint64
}

func (m unsubscribeMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of unsubscribeMessage failed"
	}
	return fmt.Sprintf("%v:%v", unsubscribeMessageType.String(), string(buf))
}

func (m *unsubscribeMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(unsubscribeMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	return buf
}

func (p *loggingParser) parseUnsubscribeMessage() (*unsubscribeMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	return &unsubscribeMessage{
		SubscribeID: subscribeID,
	}, err
}

type subscribeDoneMessage struct {
	SusbcribeID   uint64
	StatusCode    uint64
	ReasonPhrase  string
	ContentExists bool
	FinalGroup    uint64
	FinalObject   uint64
}

func (m subscribeDoneMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of subscribeDoneMessage failed"
	}
	return fmt.Sprintf("%v:%v", subscribeDoneMessageType.String(), string(buf))
}

func (m *subscribeDoneMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(subscribeDoneMessageType))
	buf = quicvarint.Append(buf, m.SusbcribeID)
	buf = quicvarint.Append(buf, m.StatusCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
	if m.ContentExists {
		buf = append(buf, 1)
		buf = quicvarint.Append(buf, m.FinalGroup)
		buf = quicvarint.Append(buf, m.FinalObject)
		return buf
	}
	buf = append(buf, 0)
	return buf
}

func (p *loggingParser) parseSubscribeDoneMessage() (*subscribeDoneMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	statusCode, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	contentExistsByte, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	var contentExists bool
	switch contentExistsByte {
	case 0:
		contentExists = false
	case 1:
		contentExists = true
	default:
		return nil, errors.New("invalid use of ContentExists byte")
	}
	if !contentExists {
		return &subscribeDoneMessage{
			SusbcribeID:   subscribeID,
			StatusCode:    statusCode,
			ReasonPhrase:  reasonPhrase,
			ContentExists: contentExists,
			FinalGroup:    0,
			FinalObject:   0,
		}, nil
	}
	finalGroup, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	finalObject, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	return &subscribeDoneMessage{
		SusbcribeID:   subscribeID,
		StatusCode:    statusCode,
		ReasonPhrase:  reasonPhrase,
		ContentExists: contentExists,
		FinalGroup:    finalGroup,
		FinalObject:   finalObject,
	}, nil
}

type announceMessage struct {
	TrackNamespace         string
	TrackRequestParameters parameters
}

func (m announceMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of announceMessage failed"
	}
	return fmt.Sprintf("%v:%v", announceMessageType.String(), string(buf))
}

func (m announceMessage) trackNamespace() string {
	return m.TrackNamespace
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

func (p *loggingParser) parseAnnounceMessage() (*announceMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	trackNamspace, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	ps, err := parseParameters(p.reader)
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
		return "json.Marshal of announceOkMessage failed"
	}
	return fmt.Sprintf("%v:%v", announceOkMessageType.String(), string(buf))
}

func (m announceOkMessage) trackNamespace() string {
	return m.TrackNamespace
}

func (m *announceOkMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceOkMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return buf
}

func (p *loggingParser) parseAnnounceOkMessage() (*announceOkMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(p.reader)
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
		return "json.Marshal of announceErrorMessage failed"
	}
	return fmt.Sprintf("%v:%v", announceErrorMessageType.String(), string(buf))
}

func (m announceErrorMessage) trackNamespace() string {
	return m.TrackNamespace
}

func (m *announceErrorMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceErrorMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	buf = quicvarint.Append(buf, m.ErrorCode)
	buf = appendVarIntString(buf, m.ReasonPhrase)
	return buf
}

func (p *loggingParser) parseAnnounceErrorMessage() (*announceErrorMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	trackNamspace, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	errorCode, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	reasonPhrase, err := parseVarIntString(p.reader)
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
		return "json.Marshal of unannounceMessage failed"
	}
	return fmt.Sprintf("%v:%v", unannounceMessageType.String(), string(buf))
}

func (m *unannounceMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(unannounceMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return buf
}

func (p *loggingParser) parseUnannounceMessage() (*unannounceMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(p.reader)
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
		return "json.Marshal of goAwayMessage failed"
	}
	return fmt.Sprintf("%v:%v", goAwayMessageType.String(), string(buf))
}

func (m *goAwayMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(goAwayMessageType))
	buf = appendVarIntString(buf, m.NewSessionURI)
	return buf
}

func (p *loggingParser) parseGoAwayMessage() (*goAwayMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	uri, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	return &goAwayMessage{
		NewSessionURI: uri,
	}, nil
}

type streamHeaderTrackMessage struct {
	SubscribeID     uint64
	TrackAlias      uint64
	ObjectSendOrder uint64
}

func (m *streamHeaderTrackMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(streamHeaderTrackMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.ObjectSendOrder)
	return buf
}

func (p *loggingParser) parseStreamHeaderTrackMessage() (*streamHeaderTrackMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	trackAlias, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	objectSendOrder, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	return &streamHeaderTrackMessage{
		SubscribeID:     subscribeID,
		TrackAlias:      trackAlias,
		ObjectSendOrder: objectSendOrder,
	}, nil
}

type streamHeaderGroupMessage struct {
	SubscribeID     uint64
	TrackAlias      uint64
	GroupID         uint64
	ObjectSendOrder uint64
}

func (m *streamHeaderGroupMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(streamHeaderGroupMessageType))
	buf = quicvarint.Append(buf, m.SubscribeID)
	buf = quicvarint.Append(buf, m.TrackAlias)
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectSendOrder)
	return buf
}

func (p *loggingParser) parseStreamHeaderGroupMessage() (*streamHeaderGroupMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	subscribeID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	trackAlias, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	groupID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	objectSendOrder, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	return &streamHeaderGroupMessage{
		SubscribeID:     subscribeID,
		TrackAlias:      trackAlias,
		GroupID:         groupID,
		ObjectSendOrder: objectSendOrder,
	}, nil
}

type streamHeaderTrackObject struct {
	GroupID       uint64
	ObjectID      uint64
	ObjectPayload []byte
}

func (m *streamHeaderTrackObject) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.GroupID)
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	buf = append(buf, m.ObjectPayload...)
	return buf
}

func (p *loggingParser) parseStreamHeaderTrackObject() (*streamHeaderTrackObject, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	groupID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	objectID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	objectLen, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, objectLen)
	_, err = io.ReadFull(p.reader, buf)
	if err != nil {
		return nil, err
	}
	return &streamHeaderTrackObject{
		GroupID:       groupID,
		ObjectID:      objectID,
		ObjectPayload: buf,
	}, nil
}

type streamHeaderGroupObject struct {
	ObjectID      uint64
	ObjectPayload []byte
}

func (m *streamHeaderGroupObject) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, m.ObjectID)
	buf = quicvarint.Append(buf, uint64(len(m.ObjectPayload)))
	buf = append(buf, m.ObjectPayload...)
	return buf
}

func (p *loggingParser) parseStreamHeaderGroupObject() (*streamHeaderGroupObject, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	objectID, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	objectLen, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, objectLen)
	_, err = io.ReadFull(p.reader, buf)
	if err != nil {
		return nil, err
	}
	return &streamHeaderGroupObject{
		ObjectID:      objectID,
		ObjectPayload: buf,
	}, nil
}

type announceCancelMessage struct {
	TrackNamespace string
}

func (m announceCancelMessage) String() string {
	buf, err := json.Marshal(m)
	if err != nil {
		return "json.Marshal of unannounceMessage failed"
	}
	return fmt.Sprintf("%v:%v", announceCancelMessageType.String(), string(buf))
}

func (m *announceCancelMessage) append(buf []byte) []byte {
	buf = quicvarint.Append(buf, uint64(announceCancelMessageType))
	buf = appendVarIntString(buf, m.TrackNamespace)
	return buf
}

func (p *loggingParser) parseAnnounceCancelMessage() (*announceCancelMessage, error) {
	if p.reader == nil {
		return nil, errInvalidMessageReader
	}
	namespace, err := parseVarIntString(p.reader)
	if err != nil {
		return nil, err
	}
	return &announceCancelMessage{
		TrackNamespace: namespace,
	}, nil
}
