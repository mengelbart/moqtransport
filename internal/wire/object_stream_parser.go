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

var (
	errInvalidStreamType = errors.New("invalid stream type")
)

type ObjectStreamParser struct {
	qlogger  *qlog.Logger
	streamID uint64

	reader messageReader
	typ    StreamType

	subscribeID       uint64
	trackAlias        uint64
	PublisherPriority uint8
	GroupID           uint64
}

func (p *ObjectStreamParser) Type() StreamType {
	return p.typ
}

func (p *ObjectStreamParser) TrackAlias() (uint64, error) {
	if p.typ != StreamTypeSubgroup {
		return 0, errors.New("only subgroup streams have a track alias")
	}
	return p.trackAlias, nil
}

func (p *ObjectStreamParser) SubscribeID() (uint64, error) {
	if p.typ != StreamTypeFetch {
		return 0, errors.New("only fetch streams have a subscribe ID")
	}
	return p.subscribeID, nil
}

func NewObjectStreamParser(r io.Reader, streamID uint64, qlogger *qlog.Logger) (*ObjectStreamParser, error) {
	br := bufio.NewReader(r)
	st, err := quicvarint.Read(br)
	if err != nil {
		return nil, err
	}
	typ := StreamType(st)
	if qlogger != nil {
		var qt moqt.StreamType
		if typ == StreamTypeFetch {
			qt = moqt.StreamTypeFetchHeader
		}
		if typ == StreamTypeSubgroup {
			qt = moqt.StreamTypeSubgroupHeader
		}
		qlogger.Log(moqt.StreamTypeSetEvent{
			Owner:      moqt.GetOwner(moqt.OwnerRemote),
			StreamID:   streamID,
			StreamType: qt,
		})
	}
	switch typ {
	case StreamTypeFetch:
		var fhm FetchHeaderMessage
		if err := fhm.parse(br); err != nil {
			return nil, err
		}
		return &ObjectStreamParser{
			qlogger:           qlogger,
			streamID:          streamID,
			reader:            br,
			typ:               typ,
			subscribeID:       fhm.SubscribeID,
			trackAlias:        0,
			PublisherPriority: 0,
			GroupID:           0,
		}, nil

	case StreamTypeSubgroup:
		var shsm StreamHeaderSubgroupMessage
		if err := shsm.parse(br); err != nil {
			return nil, err
		}
		return &ObjectStreamParser{
			qlogger:           qlogger,
			streamID:          streamID,
			reader:            br,
			typ:               typ,
			subscribeID:       0,
			trackAlias:        shsm.TrackAlias,
			PublisherPriority: shsm.PublisherPriority,
			GroupID:           shsm.GroupID,
		}, nil

	default:
		return nil, fmt.Errorf("%w: %v", errInvalidStreamType, st)
	}
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

func (p *ObjectStreamParser) Parse() (*ObjectMessage, error) {
	m := &ObjectMessage{
		TrackAlias:        p.trackAlias,
		GroupID:           p.GroupID,
		SubgroupID:        0,
		ObjectID:          0,
		PublisherPriority: p.PublisherPriority,
		ObjectStatus:      0,
		ObjectPayload:     nil,
	}
	var err error
	switch p.typ {
	case StreamTypeFetch:
		err = m.readFetch(p.reader)
	case StreamTypeSubgroup:
		err = m.readSubgroup(p.reader)
	default:
		return nil, errInvalidStreamType
	}
	if p.qlogger != nil {
		var e qlog.Event
		eth := slices.Collect(slices.Map(
			m.ObjectHeaderExtensions,
			func(e ObjectHeaderExtension) moqt.ExtensionHeader {
				return moqt.ExtensionHeader{
					HeaderType:   e.key(),
					HeaderValue:  0, // TODO
					HeaderLength: 0, // TODO
					Payload:      qlog.RawInfo{},
				}
			}),
		)
		if p.typ == StreamTypeFetch {
			e = moqt.FetchObjectEvent{
				EventName:              moqt.FetchObjectEventParsed,
				StreamID:               p.streamID,
				GroupID:                m.GroupID,
				SubgroupID:             m.SubgroupID,
				ObjectID:               m.ObjectID,
				PublisherPriority:      m.PublisherPriority,
				ExtensionHeadersLength: uint64(len(m.ObjectHeaderExtensions)),
				ExtensionHeaders:       eth,
				ObjectPayloadLength:    uint64(len(m.ObjectPayload)),
				ObjectStatus:           uint64(m.ObjectStatus),
				ObjectPayload: qlog.RawInfo{
					Length:        uint64(len(m.ObjectPayload)),
					PayloadLength: uint64(len(m.ObjectPayload)),
					Data:          m.ObjectPayload,
				},
			}
		}
		if p.typ == StreamTypeSubgroup {
			gid := new(uint64)
			sid := new(uint64)
			*gid = p.GroupID
			*sid = p.SubgroupID
			e = moqt.SubgroupObjectEvent{
				EventName:              moqt.SubgroupObjectEventParsed,
				StreamID:               p.streamID,
				GroupID:                gid,
				SubgroupID:             sid,
				ObjectID:               m.ObjectID,
				ExtensionHeadersLength: uint64(len(m.ObjectHeaderExtensions)),
				ExtensionHeaders:       eth,
				ObjectPayloadLength:    uint64(len(m.ObjectPayload)),
				ObjectStatus:           uint64(m.ObjectStatus),
				ObjectPayload: qlog.RawInfo{
					Length:        uint64(len(m.ObjectPayload)),
					PayloadLength: uint64(len(m.ObjectPayload)),
					Data:          m.ObjectPayload,
				},
			}
		}
		if e != nil {
			p.qlogger.Log(e)
		}
	}
	return m, err
}
