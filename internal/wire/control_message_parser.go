package wire

import (
	"bufio"
	"fmt"
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
	case messageTypeClientSetup:
		m = &ClientSetupMessage{}
	case messageTypeServerSetup:
		m = &ServerSetupMessage{}

	case messageTypeGoAway:
		m = &GoAwayMessage{}

	case messageTypeMaxRequestID:
		m = &MaxRequestIDMessage{}
	case messageTypeRequestsBlocked:
		m = &RequestsBlockedMessage{}

	case messageTypeSubscribe:
		m = &SubscribeMessage{}
	case messageTypeSubscribeOk:
		m = &SubscribeOkMessage{}
	case messageTypeSubscribeError:
		m = &SubscribeErrorMessage{}
	case messageTypeUnsubscribe:
		m = &UnsubscribeMessage{}
	case messageTypeSubscribeUpdate:
		m = &SubscribeUpdateMessage{}
	case messageTypeSubscribeDone:
		m = &SubscribeDoneMessage{}

	case messageTypeFetch:
		m = &FetchMessage{}
	case messageTypeFetchOk:
		m = &FetchOkMessage{}
	case messageTypeFetchError:
		m = &FetchErrorMessage{}
	case messageTypeFetchCancel:
		m = &FetchCancelMessage{}

	case messageTypeTrackStatus:
		m = &TrackStatusRequestMessage{}
	case messageTypeTrackStatusOk:
		m = &TrackStatusMessage{}

	case messageTypeAnnounce:
		m = &AnnounceMessage{}
	case messageTypeAnnounceOk:
		m = &AnnounceOkMessage{}
	case messageTypeAnnounceError:
		m = &AnnounceErrorMessage{}
	case messageTypeUnannounce:
		m = &UnannounceMessage{}
	case messageTypeAnnounceCancel:
		m = &AnnounceCancelMessage{}

	case messageTypeSubscribeNamespace:
		m = &SubscribeAnnouncesMessage{}
	case messageTypeSubscribeNamespaceOk:
		m = &SubscribeAnnouncesOkMessage{}
	case messageTypeSubscribeNamespaceError:
		m = &SubscribeAnnouncesErrorMessage{}
	case messageTypeUnsubscribeNamespace:
		m = &UnsubscribeAnnouncesMessage{}
	default:
		return nil, fmt.Errorf("%w: %v", errInvalidMessageType, mt)
	}
	err = m.parse(CurrentVersion, msg)
	return m, err
}
