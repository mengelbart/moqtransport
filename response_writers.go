package moqtransport

// SubscribeResponseWriter provides enhanced methods for responding to SUBSCRIBE messages
type SubscribeResponseWriter interface {
	ResponseWriter
	Publisher

	// AcceptDetailed sends a SUBSCRIBE_OK message with custom parameters.
	// This allows setting group order, expiration, and other response fields.
	AcceptDetailed(opts ...SubscribeOkOption) error

	// RejectDetailed sends a SUBSCRIBE_ERROR message with a specific error code
	RejectDetailed(code SubscribeError, reason string) error
}

// AnnounceResponseWriter provides enhanced methods for responding to ANNOUNCE messages
type AnnounceResponseWriter interface {
	ResponseWriter

	// AcceptDetailed sends an ANNOUNCE_OK message
	AcceptDetailed() error

	// RejectDetailed sends an ANNOUNCE_ERROR message with a specific error code
	RejectDetailed(code AnnounceError, reason string) error
}

// subscribeResponseWriterAdapter wraps the existing subscriptionResponseWriter
// to provide the enhanced interface
type subscribeResponseWriterAdapter struct {
	*subscriptionResponseWriter
}

// AcceptDetailed sends a SUBSCRIBE_OK message with custom parameters
func (w *subscribeResponseWriterAdapter) AcceptDetailed(opts ...SubscribeOkOption) error {
	// Build the message
	_, err := NewSubscribeOkMessage(w.trackAlias, opts...)
	if err != nil {
		return err
	}

	// For now, we'll use the existing Accept() method
	// TODO: Implement proper detailed response sending with the built message
	return w.Accept()
}

// RejectDetailed sends a SUBSCRIBE_ERROR message with a specific error code
func (w *subscribeResponseWriterAdapter) RejectDetailed(code SubscribeError, reason string) error {
	// Use the existing Reject method with the error code
	return w.Reject(uint64(code), reason)
}

// announceResponseWriterAdapter wraps the existing announcementResponseWriter
type announceResponseWriterAdapter struct {
	*announcementResponseWriter
}

// AcceptDetailed sends an ANNOUNCE_OK message
func (w *announceResponseWriterAdapter) AcceptDetailed() error {
	return w.Accept()
}

// RejectDetailed sends an ANNOUNCE_ERROR message with a specific error code
func (w *announceResponseWriterAdapter) RejectDetailed(code AnnounceError, reason string) error {
	return w.Reject(uint64(code), reason)
}

// TypedHandler is an enhanced handler interface that receives typed messages
type TypedHandler interface {
	HandleTyped(ResponseWriter, TypedMessage)
}

// TypedHandlerFunc is a function that implements TypedHandler
type TypedHandlerFunc func(ResponseWriter, TypedMessage)

// HandleTyped implements TypedHandler
func (f TypedHandlerFunc) HandleTyped(rw ResponseWriter, msg TypedMessage) {
	f(rw, msg)
}

// HandlerAdapter adapts a TypedHandler to work with the existing Handler interface
type HandlerAdapter struct {
	typedHandler TypedHandler
}

// NewHandlerAdapter creates a new adapter from a TypedHandler
func NewHandlerAdapter(handler TypedHandler) Handler {
	return &HandlerAdapter{typedHandler: handler}
}

// Handle converts the untyped Message to a typed message and calls the typed handler
func (a *HandlerAdapter) Handle(rw ResponseWriter, msg *Message) {
	// Convert to typed message
	typedMsg, err := messageToTyped(msg)
	if err != nil {
		// For unsupported message types, fall back to basic handling
		if rejector, ok := rw.(interface{ Reject(uint64, string) error }); ok {
			_ = rejector.Reject(0, "unsupported message type")
		}
		return
	}

	// Wrap response writer if needed
	var wrappedRW ResponseWriter = rw
	switch typedMsg.Type() {
	case MessageSubscribe:
		if srw, ok := rw.(*subscriptionResponseWriter); ok {
			wrappedRW = &subscribeResponseWriterAdapter{srw}
		}
	case MessageAnnounce:
		if arw, ok := rw.(*announcementResponseWriter); ok {
			wrappedRW = &announceResponseWriterAdapter{arw}
		}
	}

	// Call the typed handler
	a.typedHandler.HandleTyped(wrappedRW, typedMsg)
}
