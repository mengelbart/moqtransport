package moqtransport

import (
	"github.com/mengelbart/moqtransport/internal/wire"
)

type localTrack struct {
	conn        Connection
	subscribeID uint64
	trackAlias  uint64
	fetchStream SendStream
	subgroups   map[uint64]*Subgroup
}

func newLocalTrack(conn Connection, subscribeID, trackAlias uint64) *localTrack {
	publisher := &localTrack{
		conn:        conn,
		subscribeID: subscribeID,
		trackAlias:  trackAlias,
		fetchStream: nil,
		subgroups:   map[uint64]*Subgroup{},
	}
	return publisher
}

func (p *localTrack) OpenFetchStream() (*FetchStream, error) {
	stream, err := p.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	fs, err := newFetchStream(stream, p.subscribeID)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func (p *localTrack) SendDatagram(o Object) error {
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

func (p *localTrack) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*Subgroup, error) {
	stream, err := p.conn.OpenUniStream()
	if err != nil {
		return nil, err
	}
	return newSubgroup(stream, p.trackAlias, groupID, subgroupID, priority)
}

func (s *localTrack) Close() error {
	return nil
}
