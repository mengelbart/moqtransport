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

func (p *ControlMessageParser) Parse() (ControlMessage, error) {
	mt, err := quicvarint.Read(p.reader)
	if err != nil {
		return nil, err
	}
	hi, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	lo, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	length := uint16(hi)<<8 | uint16(lo)

	msg := make([]byte, length)
	n, err := io.ReadFull(p.reader, msg)
	if err != nil {
		return nil, err
	}
	if n != int(length) {
		return nil, errLengthMismatch
	}

	var m ControlMessage
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
	case messageTypeSubscribeAnnounces:
		m = &SubscribeAnnouncesMessage{}
	case messageTypeSubscribeAnnouncesOk:
		m = &SubscribeAnnouncesOkMessage{}
	case messageTypeSubscribeAnnouncesError:
		m = &SubscribeAnnouncesErrorMessage{}
	case messageTypeUnsubscribeAnnounces:
		m = &UnsubscribeAnnouncesMessage{}
	case messageTypeMaxRequestID:
		m = &MaxRequestIDMessage{}
	case messageTypeFetch:
		m = &FetchMessage{}
	case messageTypeFetchCancel:
		m = &FetchCancelMessage{}
	case messageTypeFetchOk:
		m = &FetchOkMessage{}
	case messageTypeFetchError:
		m = &FetchErrorMessage{}
	case messageTypeClientSetup:
		m = &ClientSetupMessage{}
	case messageTypeServerSetup:
		m = &ServerSetupMessage{}
	default:
		return nil, errInvalidMessageType
	}
	err = m.parse(CurrentVersion, msg)
	return m, err
}
