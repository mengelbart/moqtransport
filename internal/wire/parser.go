package wire

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/mengelbart/moqtransport/varint"
)

var errLengthMismatch = errors.New("length mismatch")

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

// validator is implemented by messages with well formedness rules the wire
// format alone cannot express.
type validator interface {
	validate() error
}

func parseMessage(m ControlMessage, r messageReader, version uint64) error {
	switch version {
	case 18:
		if err := m.parse_v18(r); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported version: %d", version)
	}
	if v, ok := m.(validator); ok {
		return v.validate()
	}
	return nil
}

type Parser struct {
	bounded      *boundedReader
	unbounded    *unboundedReader
	version      uint64
	streamType   StreamType
	objectParser *objectMessageParser
}

func NewParser(r io.Reader, version uint64, streamType StreamType) *Parser {
	reader := bufio.NewReader(r)
	return &Parser{
		bounded:      &boundedReader{reader: reader},
		unbounded:    &unboundedReader{reader: reader},
		version:      version,
		streamType:   streamType,
		objectParser: nil,
	}
}

func (p *Parser) Read() (ControlMessage, error) {
	p.unbounded.reset()

	if p.objectParser != nil {
		return p.objectParser.parse()
	}
	mt, err := varint.Read(p.unbounded)
	if err != nil {
		return nil, err
	}
	if p.streamType == StreamTypeData {
		return p.readDataHeader(mt)
	}

	length, err := p.readLength()
	if err != nil {
		return nil, err
	}

	m, err := p.controlMessage(mt)
	if err != nil {
		return nil, err
	}

	p.bounded.reset(int64(length))
	if err := parseMessage(m, p.bounded, p.version); err != nil {
		return nil, err
	}
	// A message that under-reads its body would leave the rest of it in the
	// stream and desync every message after it.
	if p.bounded.remaining() != 0 {
		if err := p.bounded.discard(); err != nil {
			return nil, err
		}
		return nil, errLengthMismatch
	}
	return m, nil
}

func (p *Parser) readLength() (uint16, error) {
	hi, err := p.unbounded.ReadByte()
	if err != nil {
		return 0, err
	}
	lo, err := p.unbounded.ReadByte()
	if err != nil {
		return 0, err
	}
	return uint16(hi)<<8 | uint16(lo), nil
}

func (p *Parser) controlMessage(mt uint64) (ControlMessage, error) {
	switch p.streamType {
	case StreamTypeControl:
		switch ControlMessageType(mt) {
		case ControlMessageTypeSetup:
			return &Setup{}, nil
		case ControlMessageTypeGoAway:
			return &GoAwayCtrl{}, nil
		}
	case StreamTypeRequest:
		switch ControlMessageType(mt) {
		case ControlMessageTypeGoAway:
			return &GoAwayReq{}, nil

		case ControlMessageTypeFetch:
			return &Fetch{}, nil
		case ControlMessageTypeFetchOk:
			return &FetchOk{}, nil

		case ControlMessageTypeNamespace:
			return &Namespace{}, nil
		case ControlMessageTypeNamespaceDone:
			return &NamespaceDone{}, nil

		case ControlMessageTypePublish:
			return &Publish{}, nil
		case ControlMessageTypePublishBlocked:
			return &PublishBlocked{}, nil
		case ControlMessageTypePublishDone:
			return &PublishDone{}, nil
		case ControlMessageTypePublishNamespace:
			return &PublishNamespace{}, nil
		case ControlMessageTypePublishOk:
			return &PublishOk{}, nil

		case ControlMessageTypeRequestError:
			return &RequestError{}, nil
		case ControlMessageTypeRequestOk:
			return &RequestOk{}, nil
		case ControlMessageTypeRequestUpdate:
			return &RequestUpdate{}, nil

		case ControlMessageTypeSubscribe:
			return &Subscribe{}, nil
		case ControlMessageTypeSubscribeNamespace:
			return &SubscribeNamespace{}, nil
		case ControlMessageTypeSubscribeOk:
			return &SubscribeOk{}, nil
		case ControlMessageTypeSubscribeTracks:
			return &SubscribeTracks{}, nil

		case ControlMessageTypeTrackStatus:
			return &TrackStatus{}, nil
		}
	default:
		return nil, fmt.Errorf("unknown stream type: %d", p.streamType)
	}
	return nil, fmt.Errorf("unknown control message type: %d", mt)
}

func (p *Parser) readDataHeader(mt uint64) (ControlMessage, error) {
	switch ControlMessageType(mt) {
	case ControlMessageTypeFetchHeader:
		m := &FetchHeader{}
		if err := parseMessage(m, p.unbounded, p.version); err != nil {
			return nil, err
		}
		// TODO: Parse fetch objects.
		return m, nil

	case ControlMessageTypePadding:
		// TODO: Discard the rest of the stream.
		return &Padding{}, nil

	default:
		m := &SubgroupHeader{
			typ: mt,
		}
		if !m.validType() {
			return nil, fmt.Errorf("unknown data stream type: %d", mt)
		}
		if err := parseMessage(m, p.unbounded, p.version); err != nil {
			return nil, err
		}
		p.objectParser = newObjectMessageParser(p.unbounded, p.version, m.Properties())
		return m, nil
	}
}

type objectMessageParser struct {
	reader        messageReader
	version       uint64
	hasProperties bool
}

func newObjectMessageParser(r messageReader, version uint64, hasProperties bool) *objectMessageParser {
	return &objectMessageParser{
		reader:        r,
		version:       version,
		hasProperties: hasProperties,
	}
}

func (p *objectMessageParser) parse() (*ObjectStream, error) {
	o := &ObjectStream{
		hasProperties: p.hasProperties,
	}
	if err := parseMessage(o, p.reader, p.version); err != nil {
		return nil, err
	}
	return o, nil
}
