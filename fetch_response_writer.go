package moqtransport

type fetchResponseWriter struct {
	id         uint64
	session    *Session
	localTrack *localTrack
}

// Accept implements ResponseWriter.
func (f *fetchResponseWriter) Accept() error {
	if err := f.session.acceptSubscription(f.id, f.localTrack); err != nil {
		return err
	}
	return nil
}

// Reject implements ResponseWriter.
func (f *fetchResponseWriter) Reject(code uint64, reason string) error {
	return f.session.rejectSubscription(f.id, code, reason)
}

func (f *fetchResponseWriter) FetchStream() (*FetchStream, error) {
	return f.localTrack.getFetchStream()
}
