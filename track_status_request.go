package moqtransport

type trackStatus struct {
	namespace    []string
	trackname    string
	statusCode   uint64
	lastGroupID  uint64
	lastObjectID uint64
}

type trackStatusRequest struct {
	namespace []string
	trackname string

	response chan *trackStatus
}
