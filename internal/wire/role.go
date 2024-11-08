package wire

import "fmt"

type Role uint64

const (
	RolePublisher Role = iota + 1
	RoleSubscriber
	RolePubSub
)

func (r Role) String() string {
	switch r {
	case RolePubSub:
		return "Pub/Sub"
	case RolePublisher:
		return "Publisher"
	case RoleSubscriber:
		return "Subscriber"
	default:
		panic(fmt.Sprintf("unexpected wire.Role: %#v", r))
	}
}
