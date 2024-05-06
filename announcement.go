package moqtransport

type Announcement struct {
	responseCh chan trackNamespacer
	namespace  string
	parameters parameters
}

func (a *Announcement) Namespace() string {
	return a.namespace
}
