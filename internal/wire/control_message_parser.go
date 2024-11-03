package wire

import (
	"bufio"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type ControlMessageParser struct {
	reader messageReader
}

func NewControlMessageParser(r io.Reader) *ControlMessageParser {
	return &ControlMessageParser{
		reader: bufio.NewReader(r),
	}
}

func (p *ControlMessageParser) Parse() (Message, error) {
	mt, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	length, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	msg := make([]byte, length)
	n, err := p.reader.Read(msg)
	if err != nil {
		return nil, err
	}
	if n != int(length) {
		return nil, errLengthMismatch
	}

	var m Message
	switch controlMessageType(mt) {
	case messageTypeSubscribeUpdate:
		m = &SubscribeUpdateMessage{}
	case messageTypeSubscribe:
		m = &SubscribeMessage{}
	case messageTypeSubscribeOk:
		m = &SubscribeOkMessage{}
	case messageTypeSubscribeError:
		m = &SubscribeErrorMessage{}
	case messageTypeAnnounce:
		m = &AnnounceMessage{}
	case messageTypeAnnounceOk:
		m = &AnnounceOkMessage{}
	case messageTypeAnnounceError:
		m = &AnnounceErrorMessage{}
	case messageTypeUnannounce:
		m = &UnannounceMessage{}
	case messageTypeUnsubscribe:
		m = &UnsubscribeMessage{}
	case messageTypeSubscribeDone:
		m = &SubscribeDoneMessage{}
	case messageTypeAnnounceCancel:
		m = &AnnounceCancelMessage{}
	case messageTypeTrackStatusRequest:
		m = &TrackStatusRequestMessage{}
	case messageTypeTrackStatus:
		m = &TrackStatusMessage{}
	case messageTypeGoAway:
		m = &GoAwayMessage{}
	case messageTypeClientSetup:
		m = &ClientSetupMessage{}
	case messageTypeServerSetup:
		m = &ServerSetupMessage{}
	default:
		return nil, errInvalidMessageType
	}
	err = m.parse(msg)
	return m, err
}
