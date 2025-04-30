package moqtransport

type TrackStatus struct {
	Namespace    []string
	Trackname    string
	StatusCode   uint64
	LastGroupID  uint64
	LastObjectID uint64
}

type trackStatusRequest struct {
	requestID uint64
	namespace []string
	trackname string

	response chan *TrackStatus
}
