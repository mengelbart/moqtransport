package wire

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/mengelbart/qlog"
	"github.com/mengelbart/qlog/moqt"
	"github.com/quic-go/quic-go/quicvarint"
)

type StreamType uint64

const (
	StreamTypeFetch                StreamType = 0x05
	StreamTypeSubgroupZeroSIDNoExt StreamType = 0x08
	StreamTypeSubgroupZeroSIDExt   StreamType = 0x09
	StreamTypeSubgroupNoSIDNoExt   StreamType = 0x0a
	StreamTypeSubgroupNoSIDExt     StreamType = 0x0b
	StreamTypeSubgroupSIDNoExt     StreamType = 0x0c
	StreamTypeSubgroupSIDExt       StreamType = 0x0d
)

var (
	errInvalidStreamType = errors.New("invalid stream type")
)

type ObjectStreamParser struct {
	qlogger  *qlog.Logger
	streamID uint64

	reader        messageReader
	typ           StreamType
	identifier    uint64 // Track Alias (Subgroup) or Request ID (Fetch)
	hasSubgroupID bool
	hasExtensions bool

	PublisherPriority uint8
	GroupID           uint64
	SubgroupID        uint64
}

func (p *ObjectStreamParser) Type() StreamType {
	return p.typ
}

func (p *ObjectStreamParser) Identifier() uint64 {
	return p.identifier
}

func NewObjectStreamParser(r io.Reader, streamID uint64, qlogger *qlog.Logger) (*ObjectStreamParser, error) {
	br := bufio.NewReader(r)
	st, err := quicvarint.Read(br)
	if err != nil {
		return nil, err
	}
	streamType := StreamType(st)

	if streamType == StreamTypeFetch {
		if qlogger != nil {
			qlogger.Log(moqt.StreamTypeSetEvent{
				Owner:      moqt.GetOwner(moqt.OwnerRemote),
				StreamID:   streamID,
				StreamType: moqt.StreamTypeFetchHeader,
			})
		}
		var fhm FetchHeaderMessage
		if err := fhm.parse(br); err != nil {
			return nil, err
		}
		return &ObjectStreamParser{
			qlogger:           qlogger,
			streamID:          streamID,
			reader:            br,
			typ:               streamType,
			identifier:        fhm.RequestID,
			PublisherPriority: 0,
			GroupID:           0,
			SubgroupID:        0,
		}, nil
	}
	if streamType >= 0x08 && streamType <= 0x0d {
		if qlogger != nil {
			qlogger.Log(moqt.StreamTypeSetEvent{
				Owner:      moqt.GetOwner(moqt.OwnerRemote),
				StreamID:   streamID,
				StreamType: moqt.StreamTypeSubgroupHeader,
			})
		}
		// least significant bit indicates if we have to read extensions on
		// objects
		ext := streamType&0x01 > 0

		// Only read subgroup ID from header if type is 0x0c or 0x0d. In all
		// other cases, it is either zero or will be read from the first object.
		sid := streamType == 0x0c || streamType == 0x0d

		var shsm SubgroupHeaderMessage
		if err := shsm.parse(br, sid); err != nil {
			return nil, err
		}
		return &ObjectStreamParser{
			qlogger:    qlogger,
			streamID:   streamID,
			reader:     br,
			typ:        streamType,
			identifier: shsm.TrackAlias,
			// if stream type is 0x0a or 0x0b, we don't yet know the subgroup ID
			// because it will only be read when the first object is parsed.
			hasSubgroupID:     streamType != 0x0a && streamType != 0x0b,
			hasExtensions:     ext,
			PublisherPriority: shsm.PublisherPriority,
			GroupID:           shsm.GroupID,
			SubgroupID:        shsm.SubgroupID,
		}, nil
	}
	return nil, fmt.Errorf("%w: %v", errInvalidStreamType, st)
}

func (p *ObjectStreamParser) Messages() iter.Seq2[*ObjectMessage, error] {
	return func(yield func(*ObjectMessage, error) bool) {
		for {
			if !yield(p.Parse()) {
				return
			}
		}
	}
}

func (p *ObjectStreamParser) parseSubgroupObject() (*ObjectMessage, error) {
	var ext KVPList
	if p.hasExtensions {
		ext = KVPList{}
	}
	m := &ObjectMessage{
		TrackAlias:             p.identifier,
		GroupID:                p.GroupID,
		SubgroupID:             p.SubgroupID,
		ObjectID:               0,
		PublisherPriority:      p.PublisherPriority,
		ObjectExtensionHeaders: ext,
		ObjectStatus:           0,
		ObjectPayload:          nil,
	}
	if err := m.readSubgroup(p.reader); err != nil {
		return nil, err
	}
	if !p.hasSubgroupID {
		p.SubgroupID = m.SubgroupID
		p.hasSubgroupID = true
	}
	if p.qlogger != nil {
		eth := slices.Collect(slices.Map(
			m.ObjectExtensionHeaders,
			func(e KeyValuePair) moqt.ExtensionHeader {
				return moqt.ExtensionHeader{
					HeaderType:   e.Type,
					HeaderValue:  0, // TODO
					HeaderLength: 0, // TODO
					Payload:      qlog.RawInfo{},
				}
			}),
		)
		gid := new(uint64)
		sid := new(uint64)
		*gid = p.GroupID
		*sid = p.SubgroupID
		p.qlogger.Log(moqt.SubgroupObjectEvent{
			EventName:              moqt.SubgroupObjectEventParsed,
			StreamID:               p.streamID,
			GroupID:                gid,
			SubgroupID:             sid,
			ObjectID:               m.ObjectID,
			ExtensionHeadersLength: uint64(len(m.ObjectExtensionHeaders)),
			ExtensionHeaders:       eth,
			ObjectPayloadLength:    uint64(len(m.ObjectPayload)),
			ObjectStatus:           uint64(m.ObjectStatus),
			ObjectPayload: qlog.RawInfo{
				Length:        uint64(len(m.ObjectPayload)),
				PayloadLength: uint64(len(m.ObjectPayload)),
				Data:          m.ObjectPayload,
			},
		})
	}
	return m, nil
}

func (p *ObjectStreamParser) parseFetchObject() (*ObjectMessage, error) {
	m := &ObjectMessage{
		TrackAlias:        0,
		GroupID:           p.GroupID,
		SubgroupID:        0,
		ObjectID:          0,
		PublisherPriority: p.PublisherPriority,
		ObjectStatus:      0,
		ObjectPayload:     nil,
	}
	if err := m.readFetch(p.reader); err != nil {
		return nil, err
	}
	if p.qlogger != nil {
		eth := slices.Collect(slices.Map(
			m.ObjectExtensionHeaders,
			func(e KeyValuePair) moqt.ExtensionHeader {
				return moqt.ExtensionHeader{
					HeaderType:   e.Type,
					HeaderValue:  0, // TODO
					HeaderLength: 0, // TODO
					Payload:      qlog.RawInfo{},
				}
			}),
		)
		p.qlogger.Log(moqt.FetchObjectEvent{
			EventName:              moqt.FetchObjectEventParsed,
			StreamID:               p.streamID,
			GroupID:                m.GroupID,
			SubgroupID:             m.SubgroupID,
			ObjectID:               m.ObjectID,
			PublisherPriority:      m.PublisherPriority,
			ExtensionHeadersLength: uint64(len(m.ObjectExtensionHeaders)),
			ExtensionHeaders:       eth,
			ObjectPayloadLength:    uint64(len(m.ObjectPayload)),
			ObjectStatus:           uint64(m.ObjectStatus),
			ObjectPayload: qlog.RawInfo{
				Length:        uint64(len(m.ObjectPayload)),
				PayloadLength: uint64(len(m.ObjectPayload)),
				Data:          m.ObjectPayload,
			},
		})
	}
	return m, nil
}

func (p *ObjectStreamParser) Parse() (*ObjectMessage, error) {
	if p.typ == StreamTypeFetch {
		return p.parseFetchObject()
	}
	if p.typ >= 0x08 && p.typ <= 0x0d {
		return p.parseSubgroupObject()
	}
	return nil, errInvalidStreamType
}
