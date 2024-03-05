package moqtransport

type announcementError struct {
	code   uint64
	reason string
}

type Announcement struct {
	errorCh chan *announcementError
	closeCh chan struct{}

	namespace  string
	parameters parameters
}

func (a *Announcement) Accept() {
	select {
	case <-a.closeCh:
	case a.errorCh <- nil:
	}
}

func (a *Announcement) Reject(code uint64, reason string) {
	select {
	case <-a.closeCh:
	case a.errorCh <- &announcementError{
		code:   code,
		reason: reason,
	}:
	}
}

func (a *Announcement) Namespace() string {
	return a.namespace
}
