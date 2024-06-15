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
	var m Message
	if err != nil {
		return nil, err
	}
	switch controlMessageType(mt) {
	case subscribeMessageType:
		m = &SubscribeMessage{}
	case subscribeUpdateMessageType:
		m = &SubscribeUpdateMessage{}
	case subscribeOkMessageType:
		m = &SubscribeOkMessage{}
	case subscribeErrorMessageType:
		m = &SubscribeErrorMessage{}
	case announceMessageType:
		m = &AnnounceMessage{}
	case announceOkMessageType:
		m = &AnnounceOkMessage{}
	case announceErrorMessageType:
		m = &AnnounceErrorMessage{}
	case unannounceMessageType:
		m = &UnannounceMessage{}
	case unsubscribeMessageType:
		m = &UnsubscribeMessage{}
	case subscribeDoneMessageType:
		m = &SubscribeDoneMessage{}
	case announceCancelMessageType:

	case trackStatusRequestMessageType:

	case trackStatusMessageType:

	case goAwayMessageType:
		m = &GoAwayMessage{}
	case clientSetupMessageType:
		m = &ClientSetupMessage{}
	case serverSetupMessageType:
		m = &ServerSetupMessage{}
	default:
		return nil, errInvalidMessageType
	}
	err = m.parse(p.reader)
	return m, err
}
