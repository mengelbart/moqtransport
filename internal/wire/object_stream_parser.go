package wire

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/quic-go/quic-go/quicvarint"
)

var (
	errInvalidStreamType = errors.New("invalid stream type")
)

type ObjectStreamParser struct {
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

func NewObjectStreamParser(r io.Reader) (*ObjectStreamParser, error) {
	br := bufio.NewReader(r)
	st, err := quicvarint.Read(br)
	if err != nil {
		return nil, err
	}
	switch StreamType(st) {
	case StreamTypeFetch:
		var fhm FetchHeaderMessage
		if err := fhm.parse(br); err != nil {
			return nil, err
		}
		return &ObjectStreamParser{
			reader:            br,
			typ:               StreamType(st),
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
			reader:            br,
			typ:               StreamType(st),
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
	return m, err
}
