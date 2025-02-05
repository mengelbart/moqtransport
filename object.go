package moqtransport

type Object struct {
	GroupID    uint64
	SubGroupID uint64
	ObjectID   uint64
	Payload    []byte
}
