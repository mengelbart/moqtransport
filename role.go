package moqtransport

type Role uint64

const (
	RolePublisher Role = iota + 1
	RoleSubscriber
	RolePubSub
)
