package wire

type GroupOrder int

const (
	GroupOrderOriginalPublisher GroupOrder = iota
	GroupOrderAscending
	GroupOrderDescending
)
