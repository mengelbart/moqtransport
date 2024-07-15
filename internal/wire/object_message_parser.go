package wire

import (
	"bufio"
	"io"

	"github.com/quic-go/quic-go/quicvarint"
)

type ObjectStreamParser struct {
	reader     messageReader
	gotHeader  bool
	streamType ObjectMessageType

	subscribeID       uint64
	trackAlias        uint64
	publisherPriority uint8
	groupID           uint64
}

func NewObjectStreamParser(r io.Reader) *ObjectStreamParser {
	return &ObjectStreamParser{
		reader:            bufio.NewReader(r),
		gotHeader:         false,
		streamType:        0,
		subscribeID:       0,
		trackAlias:        0,
		publisherPriority: 0,
		groupID:           0,
	}
}

func (p *ObjectStreamParser) Parse() (*ObjectMessage, error) {
	if !p.gotHeader {
		mt, err := quicvarint.Read(p.reader)
		if err != nil {
			return nil, err
		}
		p.streamType = ObjectMessageType(mt)
		p.gotHeader = true
		switch p.streamType {
		case StreamHeaderTrackMessageType:
			shtm := &StreamHeaderTrackMessage{}
			if err := shtm.parse(p.reader); err != nil {
				return nil, err
			}
			p.subscribeID = shtm.SubscribeID
			p.trackAlias = shtm.TrackAlias
			p.publisherPriority = shtm.PublisherPriority
		case StreamHeaderGroupMessageType:
			shgm := &StreamHeaderGroupMessage{}
			if err := shgm.parse(p.reader); err != nil {
				return nil, err
			}
			p.subscribeID = shgm.SubscribeID
			p.trackAlias = shgm.TrackAlias
			p.publisherPriority = shgm.PublisherPriority
			p.groupID = shgm.GroupID
		}
	}

	switch p.streamType {
	case ObjectStreamMessageType:
		om := &ObjectMessage{
			Type: ObjectStreamMessageType,
		}
		var buf []byte
		buf, err := io.ReadAll(p.reader)
		if err != nil {
			return nil, err
		}
		_, err = om.parse(buf)
		return om, err

	case ObjectDatagramMessageType:
		om := &ObjectMessage{
			Type: ObjectDatagramMessageType,
		}
		var buf []byte
		buf, err := io.ReadAll(p.reader)
		if err != nil {
			return nil, err
		}
		if _, err = om.parse(buf); err != nil {
			return nil, err
		}
		return om, nil

	case StreamHeaderTrackMessageType:
		om := &StreamHeaderTrackObject{}
		if err := om.parse(p.reader); err != nil {
			return nil, err
		}
		return &ObjectMessage{
			Type:              StreamHeaderTrackMessageType,
			SubscribeID:       p.subscribeID,
			TrackAlias:        p.trackAlias,
			GroupID:           om.GroupID,
			ObjectID:          om.ObjectID,
			PublisherPriority: p.publisherPriority,
			ObjectPayload:     om.ObjectPayload,
		}, nil

	case StreamHeaderGroupMessageType:
		om := &StreamHeaderGroupObject{}
		if err := om.parse(p.reader); err != nil {
			return nil, err
		}
		return &ObjectMessage{
			Type:              StreamHeaderGroupMessageType,
			SubscribeID:       p.subscribeID,
			TrackAlias:        p.trackAlias,
			GroupID:           p.groupID,
			ObjectID:          om.ObjectID,
			PublisherPriority: p.publisherPriority,
			ObjectPayload:     om.ObjectPayload,
		}, nil
	}
	return nil, errInvalidMessageType
}
