package moqtransport

type trackStatusResponseWriter struct {
	session *Session
	handled bool
	status  TrackStatus
}

// Accept commits the status and sends a response to the peer.
func (w *trackStatusResponseWriter) Accept() error {
	w.handled = true
	return w.session.sendTrackStatus(w.status)
}

// Reject sends a track does not exist status
func (w *trackStatusResponseWriter) Reject(uint64, string) error {
	w.handled = true
	w.status.StatusCode = TrackStatusDoesNotExist
	w.status.LastGroupID = 0
	w.status.LastObjectID = 0
	return w.Accept()
}

// SetStatus implements StatusRequestHandler.
func (w *trackStatusResponseWriter) SetStatus(statusCode uint64, lastGroupID uint64, lastObjectID uint64) {
	w.status.StatusCode = statusCode
	w.status.LastGroupID = lastGroupID
	w.status.LastObjectID = lastObjectID
}
