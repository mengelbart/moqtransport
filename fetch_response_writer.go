package moqtransport

import (
	"errors"
	"sync"
)

var errFetchNotAccepted = errors.New("publish before fetch accepted")

type fetchResponseWriter struct {
	id         uint64
	transport  *Transport
	lock       sync.Mutex
	localTrack *localTrack
}

// Accept implements ResponseWriter.
func (f *fetchResponseWriter) Accept() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.localTrack = newLocalTrack(f.transport.conn, f.id, 0)
	if err := f.transport.acceptSubscription(f.id, f.localTrack); err != nil {
		return err
	}
	return nil
}

// Reject implements ResponseWriter.
func (f *fetchResponseWriter) Reject(code uint64, reason string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.transport.rejectSubscription(f.id, code, reason)
}

func (f *fetchResponseWriter) OpenFetchStream() (*FetchStream, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.localTrack == nil {
		return nil, errFetchNotAccepted
	}
	return f.localTrack.OpenFetchStream()
}
