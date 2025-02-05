package moqtransport

import "github.com/mengelbart/moqtransport/internal/wire"

// Role is the MoQ role of an endpoint.
type Role = wire.Role

const (
	RolePublisher  Role = wire.RolePublisher
	RoleSubscriber Role = wire.RoleSubscriber
	RolePubSub     Role = wire.RolePubSub
)
