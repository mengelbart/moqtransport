package moqtransport

type Subscription struct {
	ID            uint64
	TrackAlias    uint64
	Namespace     string
	TrackName     string
	Authorization string
}
