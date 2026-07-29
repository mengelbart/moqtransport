package wire

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

type StreamType uint8

const (
	StreamTypeControl StreamType = iota
	StreamTypeRequest
	StreamTypeData
)

type Parser struct {
	reader       streamReader
	version      uint64
	streamType   StreamType
	objectParser *objectMessageParser
}

func NewParser(r io.Reader, version uint64, streamType StreamType) *Parser {
	return &Parser{
		reader:       bufio.NewReader(r),
		version:      version,
		streamType:   streamType,
		objectParser: nil,
	}
}

func (p *Parser) Read() (ControlMessage, error) {
	if p.objectParser != nil {
		return p.objectParser.parse()
	}
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

	switch p.streamType {
	case StreamTypeControl:
		switch ControlMessageType(mt) {
		case ControlMessageTypeSetup:
			m = &Setup{}
		case ControlMessageTypeGoAway:
			m = &GoAway{}
		default:
			return nil, fmt.Errorf("unknown control message type: %d", mt)
		}
	case StreamTypeRequest:
		switch ControlMessageType(mt) {
		case ControlMessageTypeFetch:
			m = &Fetch{}
		case ControlMessageTypeFetchOk:
			m = &FetchOk{}

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
	case StreamTypeData:
		switch ControlMessageType(mt) {
		case ControlMessageTypeFetchHeader:
			m = &FetchHeader{}
		case ControlMessageTypePadding:
			m = &Padding{}

		default:
			sgh := &SubgroupHeader{
				typ: mt & 0xff,
			}
			if !sgh.validType() {
				return nil, fmt.Errorf("unknown control message type: %d", mt)
			}
			p.objectParser = newObjectMessageParser(p.reader)
			m = sgh
		}
	default:
		panic("unknown stream type")
	}

	switch p.version {
	case 18:
		err = m.parse_v18(msg)
	default:
		return nil, fmt.Errorf("unsupported version: %d", p.version)
	}

	return m, err
}

type objectMessageParser struct {
	reader streamReader
}

func newObjectMessageParser(r streamReader) *objectMessageParser {
	return &objectMessageParser{
		reader: r,
	}
}

func (p *objectMessageParser) parse() (*ObjectStream, error) {
	o := &ObjectStream{}
	if err := o.parse(p.reader); err != nil {
		return nil, err
	}
	return o, nil
}
