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
	return f.session.acceptFetch(f.id)
}

// Reject implements ResponseWriter.
func (f *fetchResponseWriter) Reject(code uint64, reason string) error {
	f.handled = true
	return f.session.rejectFetch(f.id, code, reason)
}

func (f *fetchResponseWriter) FetchStream() (*FetchStream, error) {
	return f.localTrack.getFetchStream()
}

// Session returns the session associated with this fetch response writer.
func (f *fetchResponseWriter) Session() *Session {
	return f.session
}
