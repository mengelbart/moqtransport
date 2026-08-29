package moqtransport

type ObjectForwardingPreference int

const (
	ObjectForwardingPreferenceSubgroup ObjectForwardingPreference = iota
	ObjectForwardingPreferenceDatagram
)

// An Object is a MoQ Object.
type Object struct {
	GroupID              uint64
	ObjectID             uint64
	ForwardingPreference ObjectForwardingPreference
	SubGroupID           uint64
	Payload              []byte
}
