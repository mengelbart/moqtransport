package wire2

import (
	"bufio"
	"fmt"
	"io"

	"github.com/mengelbart/moqtransport/varint"
)

type streamReader interface {
	io.Reader
	io.ByteReader
}

type Parser struct {
	reader  streamReader
	version uint64
}

func NewParser(r io.Reader, version uint64) *Parser {
	return &Parser{
		reader:  bufio.NewReader(r),
		version: version,
	}
}

func (p *Parser) Read() (ControlMessage, error) {
	mt, err := varint.Read(p.reader)
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
	switch ControlMessageType(mt) {
	case ControlMessageTypeFetch:
		m = &Fetch{}
	case ControlMessageTypeFetchOk:
		m = &FetchOk{}

	case ControlMessageTypeGoAway:
		m = &GoAway{}

	case ControlMessageTypeNamespace:
		m = &Namespace{}
	case ControlMessageTypeNamespaceDone:
		m = &NamespaceDone{}

	case ControlMessageTypePublish:
		m = &Publish{}
	case ControlMessageTypePublishBlocked:
		m = &PublishBlocked{}
	case ControlMessageTypePublishDone:
		m = &PublishDone{}
	case ControlMessageTypePublishNamespace:
		m = &PublishNamespace{}
	case ControlMessageTypePublishOk:
		m = &PublishOk{}

	case ControlMessageTypeRequestError:
		m = &RequestError{}
	case ControlMessageTypeRequestOk:
		m = &RequestOk{}
	case ControlMessageTypeRequestUpdate:
		m = &RequestUpdate{}

	case ControlMessageTypeSetup:
		m = &Setup{}

	case ControlMessageTypeSubscribe:
		m = &Subscribe{}
	case ControlMessageTypeSubscribeNamespace:
		m = &SubscribeNamespace{}
	case ControlMessageTypeSubscribeOk:
		m = &SubscribeOk{}
	case ControlMessageTypeSubscribeTracks:
		m = &SubscribeTracks{}

	case ControlMessageTypeTrackStatus:
		m = &TrackStatus{}

	default:
		return nil, fmt.Errorf("unknown control message type: %d", mt)
	}

	switch p.version {
	case 18:
		err = m.parse_v18(msg)
	default:
		return nil, fmt.Errorf("unsupported version: %d", p.version)
	}

	return m, err
}
