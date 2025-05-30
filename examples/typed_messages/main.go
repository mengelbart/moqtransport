// Package main demonstrates the new typed message handling pattern
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mengelbart/moqtransport"
)

// ExampleTypedHandler demonstrates using the new typed message handling pattern
type ExampleTypedHandler struct{}

// HandleTyped processes typed messages with full access to all fields
func (h *ExampleTypedHandler) HandleTyped(rw moqtransport.ResponseWriter, msg moqtransport.TypedMessage) {
	switch m := msg.(type) {
	case *moqtransport.SubscribeMessage:
		// Access all subscribe fields directly
		log.Printf("Subscribe request for %s/%s", m.TrackNamespace, m.TrackName)
		log.Printf("  Track Alias: %d", m.TrackAlias)
		log.Printf("  Priority: %d", m.SubscriberPriority)
		log.Printf("  Group Order: %v", m.GroupOrder)
		log.Printf("  Filter Type: %v", m.FilterType)
		
		// Check authorization from parameters
		if auth, ok := m.Parameters["authorization"].(string); ok {
			log.Printf("  Authorization: %s", auth)
		}
		
		// Use enhanced response writer for detailed responses
		if srw, ok := rw.(moqtransport.SubscribeResponseWriter); ok {
			// Accept with custom parameters
			err := srw.AcceptDetailed(
				moqtransport.WithGroupOrderResponse(moqtransport.GroupOrderDescending),
				moqtransport.WithContentExists(true),
				moqtransport.WithExpires(60000), // 60 seconds
			)
			if err != nil {
				log.Printf("Failed to accept subscription: %v", err)
				return
			}
			
			// Now we can publish content
			// srw implements Publisher interface
		} else {
			// Fallback to basic accept
			rw.Accept()
		}
		
	case *moqtransport.SubscribeUpdateMessage:
		log.Printf("Subscribe update for track alias %d", m.TrackAlias)
		log.Printf("  New priority: %d", m.SubscriberPriority)
		// No response needed for SUBSCRIBE_UPDATE
		
	case *moqtransport.AnnounceMessage:
		log.Printf("Announce for namespace: %s", m.TrackNamespace)
		
		if arw, ok := rw.(moqtransport.AnnounceResponseWriter); ok {
			// Reject with specific error code
			err := arw.RejectDetailed(
				moqtransport.AnnounceErrorUnauthorized,
				"Publishers not accepted on this endpoint",
			)
			if err != nil {
				log.Printf("Failed to reject announcement: %v", err)
			}
		} else {
			rw.Reject(1, "Publishers not accepted")
		}
		
	case *moqtransport.UnsubscribeMessage:
		log.Printf("Unsubscribe for track alias %d", m.TrackAlias)
		// Handle unsubscribe...
		
	default:
		log.Printf("Unhandled message type: %s", msg.Type())
	}
}

// Example of creating and sending messages from a client
func clientExample(session *moqtransport.Session) error {
	// Create a detailed subscribe message
	subscribeMsg, err := moqtransport.NewSubscribeMessage(
		moqtransport.WithTrackNamespace("example-namespace"),
		moqtransport.WithTrackName("video"),
		moqtransport.WithTrackAlias(1),
		moqtransport.WithSubscriberPriority(128),
		moqtransport.WithGroupOrder(moqtransport.GroupOrderAscending),
		moqtransport.WithFilterType(moqtransport.FilterTypeLatestGroup),
		moqtransport.WithParameters(moqtransport.Parameters{
			"custom-param": "value",
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to create subscribe message: %w", err)
	}
	
	// TODO: When session is updated, we would send like this:
	// err = session.SendMessage(subscribeMsg)
	
	// For now, use the existing Subscribe method
	// Note: Subscribe still uses namespace as []string
	_, err = session.Subscribe(
		context.Background(),
		[]string{subscribeMsg.TrackNamespace},
		subscribeMsg.TrackName,
		"", // authorization
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	
	log.Printf("Subscribed to %s/%s", subscribeMsg.TrackNamespace, subscribeMsg.TrackName)
	return nil
}

func main() {
	// Create a typed handler
	typedHandler := &ExampleTypedHandler{}
	
	// Adapt it to work with the existing Handler interface
	handler := moqtransport.NewHandlerAdapter(typedHandler)
	
	// Use the handler with a server
	// server := moqtransport.NewServer(handler, ...)
	
	// Or use directly with a session
	// session.SetHandler(handler)
	
	log.Printf("Handler created: %v", handler)
}