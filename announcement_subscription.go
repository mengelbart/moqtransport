package moqtransport

type announcementSubscriptionResponse struct {
	err error
}

type announcementSubscription struct {
	requestID uint64
	namespace []string
	response  chan announcementSubscriptionResponse
}
