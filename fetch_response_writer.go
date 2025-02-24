package moqtransport

type fetchResponseWriter struct {
	id         uint64
	session    *Session
	localTrack *localTrack
	handled    bool
}

// Accept implements ResponseWriter.
func (f *fetchResponseWriter) Accept() error {
	f.handled = true
	return f.session.acceptSubscription(f.id, f.localTrack)
}

// Reject implements ResponseWriter.
func (f *fetchResponseWriter) Reject(code uint64, reason string) error {
	f.handled = true
	return f.session.rejectSubscription(f.id, code, reason)
}

func (f *fetchResponseWriter) FetchStream() (*FetchStream, error) {
	return f.localTrack.getFetchStream()
}
