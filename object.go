package moqtransport

type ObjectForwardingPreference int

const (
	ObjectForwardingPreferenceNone ObjectForwardingPreference = iota
	ObjectForwardingPreferenceDatagram
	ObjectForwardingPreferenceSubgroup
)

type Object struct {
	GroupID    uint64
	SubGroupID uint64
	ObjectID   uint64
	Payload    []byte
}
