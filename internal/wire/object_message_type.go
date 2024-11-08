package wire

type ObjectMessageType uint64

// TODO: These are named inconsistently (Why FetchHeader but not
// subgroupHeader?)
const (
	ObjectDatagramMessageType       ObjectMessageType = 0x01
	StreamHeaderSubgroupMessageType ObjectMessageType = 0x04
)

func (mt ObjectMessageType) String() string {
	switch mt {
	case ObjectDatagramMessageType:
		return "objectDatagram"
	case StreamHeaderSubgroupMessageType:
		return "streamHeaderSubgroupMessageType"
	}
	return "unknown message type"
}
