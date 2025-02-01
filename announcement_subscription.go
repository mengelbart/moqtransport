package moqtransport

type announcementSubscriptionResponse struct {
	err error
}

type announcementSubscription struct {
	namespace []string
	response  chan announcementSubscriptionResponse
}
