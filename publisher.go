package moqtransport

import (
	"github.com/mengelbart/moqtransport/internal/wire"
)

type LocalTrack struct {
	conn        Connection
	subscribeID uint64
	trackAlias  uint64
	subgroups   map[uint64]*Subgroup
}

func newLocalTrack(conn Connection, subscribeID, trackAlias uint64) *LocalTrack {
	publisher := &LocalTrack{
		conn:        conn,
		subscribeID: subscribeID,
		trackAlias:  trackAlias,
		subgroups:   map[uint64]*Subgroup{},
	}
	return publisher
}

func (p *LocalTrack) SendDatagram(o Object) error {
	om := &wire.ObjectMessage{
		TrackAlias:        0,
		GroupID:           o.GroupID,
		SubgroupID:        o.SubGroupID,
		ObjectID:          o.ObjectID,
		PublisherPriority: 0,
		ObjectStatus:      0,
		ObjectPayload:     o.Payload,
	}
	buf := make([]byte, 0, 48+len(o.Payload))
	buf = om.AppendDatagram(buf)
	return p.conn.SendDatagram(buf)
}

func (p *LocalTrack) OpenSubgroup(groupID uint64, priority uint8) (*Subgroup, error) {
	stream, err := p.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newSubgroup(stream, p.subscribeID, p.trackAlias, groupID, priority)
}

func (s *LocalTrack) Close() error {
	panic("TODO")
}
