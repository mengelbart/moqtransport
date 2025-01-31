package moqtransport

type announcementSubscriptionResponse struct {
	err error
}

type AnnouncementSubscription struct {
	namespace []string
	response  chan announcementSubscriptionResponse
}
