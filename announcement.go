package moqtransport

type Announcement struct {
	responseCh chan error
	closeCh    chan struct{}

	namespace  string
	parameters parameters
}

func (a *Announcement) Accept() {
	select {
	case <-a.closeCh:
	case a.responseCh <- nil:
	}
}

func (a *Announcement) Reject(err error) {
	select {
	case <-a.closeCh:
	case a.responseCh <- err:
	}
}

func (a *Announcement) Namespace() string {
	return a.namespace
}
