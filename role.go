package moqtransport

type Role uint64

const (
	IngestionRole Role = iota + 1
	DeliveryRole
	IngestionDeliveryRole
)
