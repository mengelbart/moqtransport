package moqtransport

type Announcement struct {
	errorCh chan error
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

func (a *Announcement) Reject(err error) {
	select {
	case <-a.closeCh:
	case a.errorCh <- err:
	}
}

func (a *Announcement) Namespace() string {
	return a.namespace
}
