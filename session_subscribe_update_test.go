package moqtransport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func TestSessionSubscribeUpdate(t *testing.T) {
	t.Run("onSubscribeUpdate_unknown_request_id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Try to update a subscription that doesn't exist
		err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
			RequestID:          999,
			StartLocation:      wire.Location{Group: 10, Object: 5},
			EndGroup:           20,
			SubscriberPriority: 50,
			Forward:            1,
			Parameters:         wire.KVPList{},
		})
		
		assert.Equal(t, errUnknownRequestID, err)
	})

	t.Run("onSubscribeUpdate_success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// First, add a local track to simulate an active subscription
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		s.localTracks.confirm(42) // Move to open state
		
		// Expect the message to be enqueued for the application handler
		mh.EXPECT().enqueue(context.Background(), &Message{
			Method:             MessageSubscribeUpdate,
			RequestID:          42,
			TrackAlias:         100,
			SubscriberPriority: 50,
			StartGroup:         10,
			StartObject:        5,
			EndGroup:           20,
			Forward:            1,
		}).Return(nil)
		
		// Send SUBSCRIBE_UPDATE
		err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
			RequestID:          42,
			StartLocation:      wire.Location{Group: 10, Object: 5},
			EndGroup:           20,
			SubscriberPriority: 50,
			Forward:            1,
			Parameters:         wire.KVPList{},
		})
		
		assert.NoError(t, err)
	})

	t.Run("onSubscribeUpdate_pending_subscription", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Add a pending local track (not yet confirmed)
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		// Don't confirm it - leave it in pending state
		
		// Should still find the track since findByID checks both pending and open
		mh.EXPECT().enqueue(context.Background(), gomock.Any()).Return(nil)
		
		err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
			RequestID:          42,
			StartLocation:      wire.Location{Group: 10, Object: 5},
			EndGroup:           20,
			SubscriberPriority: 50,
			Forward:            1,
			Parameters:         wire.KVPList{},
		})
		
		assert.NoError(t, err)
	})

	t.Run("receive_subscribe_update_message", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Add a local track
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		s.localTracks.confirm(42)
		
		// Mark handshake as done
		close(s.handshakeDoneCh)
		
		// Expect the message to be passed to application
		mh.EXPECT().enqueue(context.Background(), gomock.Any()).Return(nil)
		
		// Simulate receiving SUBSCRIBE_UPDATE via control stream
		err := s.receive(&wire.SubscribeUpdateMessage{
			RequestID:          42,
			StartLocation:      wire.Location{Group: 10, Object: 5},
			EndGroup:           20,
			SubscriberPriority: 50,
			Forward:            1,
			Parameters:         wire.KVPList{},
		})
		
		assert.NoError(t, err)
	})

	t.Run("subscribe_update_priority_values", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Add a local track
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		s.localTracks.confirm(42)
		
		// Test various priority values (0-255)
		priorities := []uint8{0, 1, 127, 128, 255}
		
		for _, priority := range priorities {
			mh.EXPECT().enqueue(context.Background(), &Message{
				Method:             MessageSubscribeUpdate,
				RequestID:          42,
				TrackAlias:         100,
				SubscriberPriority: priority,
				StartGroup:         0,
				StartObject:        0,
				EndGroup:           0,
				Forward:            1,
			}).Return(nil)
			
			err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
				RequestID:          42,
				StartLocation:      wire.Location{Group: 0, Object: 0},
				EndGroup:           0,
				SubscriberPriority: priority,
				Forward:            1,
				Parameters:         wire.KVPList{},
			})
			
			assert.NoError(t, err, "Priority %d should be accepted", priority)
		}
	})

	t.Run("subscribe_update_forward_flag", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Add a local track
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		s.localTracks.confirm(42)
		
		// Test pause (Forward=0)
		mh.EXPECT().enqueue(context.Background(), &Message{
			Method:             MessageSubscribeUpdate,
			RequestID:          42,
			TrackAlias:         100,
			SubscriberPriority: 128,
			StartGroup:         0,
			StartObject:        0,
			EndGroup:           0,
			Forward:            0, // Pause
		}).Return(nil)
		
		err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
			RequestID:          42,
			StartLocation:      wire.Location{Group: 0, Object: 0},
			EndGroup:           0,
			SubscriberPriority: 128,
			Forward:            0, // Pause
			Parameters:         wire.KVPList{},
		})
		require.NoError(t, err)
		
		// Test resume (Forward=1)
		mh.EXPECT().enqueue(context.Background(), &Message{
			Method:             MessageSubscribeUpdate,
			RequestID:          42,
			TrackAlias:         100,
			SubscriberPriority: 128,
			StartGroup:         0,
			StartObject:        0,
			EndGroup:           0,
			Forward:            1, // Resume
		}).Return(nil)
		
		err = s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
			RequestID:          42,
			StartLocation:      wire.Location{Group: 0, Object: 0},
			EndGroup:           0,
			SubscriberPriority: 128,
			Forward:            1, // Resume
			Parameters:         wire.KVPList{},
		})
		require.NoError(t, err)
	})

	t.Run("subscribe_update_location_changes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Add a local track
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		s.localTracks.confirm(42)
		
		// Test location update
		mh.EXPECT().enqueue(context.Background(), &Message{
			Method:             MessageSubscribeUpdate,
			RequestID:          42,
			TrackAlias:         100,
			SubscriberPriority: 128,
			StartGroup:         100,
			StartObject:        50,
			EndGroup:           200,
			Forward:            1,
		}).Return(nil)
		
		err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
			RequestID:          42,
			StartLocation:      wire.Location{Group: 100, Object: 50},
			EndGroup:           200,
			SubscriberPriority: 128,
			Forward:            1,
			Parameters:         wire.KVPList{},
		})
		
		assert.NoError(t, err)
	})

	t.Run("subscribe_update_with_end_group", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Add a local track
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		s.localTracks.confirm(42)
		
		// Test various EndGroup scenarios
		testCases := []struct {
			name        string
			startGroup  uint64
			startObject uint64
			endGroup    uint64
			description string
		}{
			{
				name:        "end_before_start",
				startGroup:  100,
				startObject: 0,
				endGroup:    50,
				description: "EndGroup before StartGroup (should be invalid in real use)",
			},
			{
				name:        "end_equals_start",
				startGroup:  100,
				startObject: 0,
				endGroup:    100,
				description: "EndGroup equals StartGroup (single group)",
			},
			{
				name:        "end_after_start",
				startGroup:  100,
				startObject: 0,
				endGroup:    500,
				description: "Normal case: EndGroup after StartGroup",
			},
			{
				name:        "large_end_value",
				startGroup:  100,
				startObject: 0,
				endGroup:    1<<63 - 1,
				description: "Maximum uint64 value for EndGroup",
			},
			{
				name:        "zero_end_value",
				startGroup:  0,
				startObject: 0,
				endGroup:    0,
				description: "All zeros (beginning of stream)",
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mh.EXPECT().enqueue(context.Background(), &Message{
					Method:             MessageSubscribeUpdate,
					RequestID:          42,
					TrackAlias:         100,
					SubscriberPriority: 128,
					StartGroup:         tc.startGroup,
					StartObject:        tc.startObject,
					EndGroup:           tc.endGroup,
					Forward:            1,
				}).Return(nil)
				
				err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
					RequestID:          42,
					StartLocation:      wire.Location{Group: tc.startGroup, Object: tc.startObject},
					EndGroup:           tc.endGroup,
					SubscriberPriority: 128,
					Forward:            1,
					Parameters:         wire.KVPList{},
				})
				
				assert.NoError(t, err, tc.description)
			})
		}
	})
	
	t.Run("subscribe_update_narrowing_subscription", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		
		// Add a local track
		lt := &localTrack{
			requestID:  42,
			trackAlias: 100,
		}
		s.localTracks.addPending(lt)
		s.localTracks.confirm(42)
		
		// Simulate narrowing the subscription over time
		// According to spec, start can only increase and end can only decrease
		updates := []struct {
			startGroup  uint64
			startObject uint64
			endGroup    uint64
			comment     string
		}{
			{0, 0, 1000, "Initial wide range"},
			{50, 0, 1000, "Move start forward"},
			{50, 0, 800, "Reduce end"},
			{100, 25, 800, "Move start forward more"},
			{100, 25, 500, "Reduce end more"},
			{200, 0, 300, "Narrow to small range"},
		}
		
		for i, update := range updates {
			mh.EXPECT().enqueue(context.Background(), &Message{
				Method:             MessageSubscribeUpdate,
				RequestID:          42,
				TrackAlias:         100,
				SubscriberPriority: 128,
				StartGroup:         update.startGroup,
				StartObject:        update.startObject,
				EndGroup:           update.endGroup,
				Forward:            1,
			}).Return(nil)
			
			err := s.onSubscribeUpdate(&wire.SubscribeUpdateMessage{
				RequestID:          42,
				StartLocation:      wire.Location{Group: update.startGroup, Object: update.startObject},
				EndGroup:           update.endGroup,
				SubscriberPriority: 128,
				Forward:            1,
				Parameters:         wire.KVPList{},
			})
			
			assert.NoError(t, err, "Update %d: %s", i, update.comment)
		}
	})
}

// Test that MessageSubscribeUpdate constant is properly defined
func TestMessageSubscribeUpdateConstant(t *testing.T) {
	// Verify the constant exists and has the expected value
	assert.Equal(t, "SUBSCRIBE_UPDATE", MessageSubscribeUpdate)
}

// Test that Message struct has all SUBSCRIBE_UPDATE fields
func TestMessageSubscribeUpdateFields(t *testing.T) {
	msg := &Message{
		Method:             MessageSubscribeUpdate,
		RequestID:          42,
		TrackAlias:         100,
		SubscriberPriority: 50,
		StartGroup:         10,
		StartObject:        5,
		EndGroup:           20,
		Forward:            1,
	}
	
	assert.Equal(t, MessageSubscribeUpdate, msg.Method)
	assert.Equal(t, uint64(42), msg.RequestID)
	assert.Equal(t, uint64(100), msg.TrackAlias)
	assert.Equal(t, uint8(50), msg.SubscriberPriority)
	assert.Equal(t, uint64(10), msg.StartGroup)
	assert.Equal(t, uint64(5), msg.StartObject)
	assert.Equal(t, uint64(20), msg.EndGroup)
	assert.Equal(t, uint8(1), msg.Forward)
}

// Test the Session.SubscribeUpdate method
func TestSessionSubscribeUpdateMethod(t *testing.T) {
	t.Run("subscribe_update_unknown_request_id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		
		// Mark handshake as done
		close(s.handshakeDoneCh)
		
		// Try to update a subscription that doesn't exist
		err := s.SubscribeUpdate(context.Background(), 999, 50, 10, 5, 20, 1)
		
		assert.Equal(t, errUnknownRequestID, err)
	})
	
	t.Run("subscribe_update_success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		
		// Mark handshake as done
		close(s.handshakeDoneCh)
		
		// Add a remote track to simulate an active subscription
		rt := newRemoteTrack(42, nil)
		err := s.remoteTracks.addPending(42, rt)
		require.NoError(t, err)
		s.remoteTracks.confirm(42)
		
		// Expect the SUBSCRIBE_UPDATE message to be sent
		cms.EXPECT().enqueue(gomock.Any(), &wire.SubscribeUpdateMessage{
			RequestID: 42,
			StartLocation: wire.Location{
				Group:  100,
				Object: 50,
			},
			EndGroup:           500,
			SubscriberPriority: 25,
			Forward:            1,
			Parameters:         wire.KVPList{},
		}).Return(nil)
		
		// Send SUBSCRIBE_UPDATE
		err = s.SubscribeUpdate(context.Background(), 42, 25, 100, 50, 500, 1)
		
		assert.NoError(t, err)
	})
	
	t.Run("subscribe_update_various_parameters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		
		// Mark handshake as done
		close(s.handshakeDoneCh)
		
		// Add a remote track
		rt := newRemoteTrack(42, nil)
		err := s.remoteTracks.addPending(42, rt)
		require.NoError(t, err)
		s.remoteTracks.confirm(42)
		
		testCases := []struct {
			name        string
			priority    uint8
			startGroup  uint64
			startObject uint64
			endGroup    uint64
			forward     uint8
		}{
			{"pause_delivery", 128, 0, 0, 0, 0},
			{"resume_delivery", 128, 0, 0, 0, 1},
			{"high_priority", 0, 0, 0, 0, 1},
			{"low_priority", 255, 0, 0, 0, 1},
			{"with_locations", 128, 100, 50, 200, 1},
			{"max_values", 255, ^uint64(0), ^uint64(0), ^uint64(0), 1},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cms.EXPECT().enqueue(gomock.Any(), &wire.SubscribeUpdateMessage{
					RequestID: 42,
					StartLocation: wire.Location{
						Group:  tc.startGroup,
						Object: tc.startObject,
					},
					EndGroup:           tc.endGroup,
					SubscriberPriority: tc.priority,
					Forward:            tc.forward,
					Parameters:         wire.KVPList{},
				}).Return(nil)
				
				err := s.SubscribeUpdate(context.Background(), 42, tc.priority, 
					tc.startGroup, tc.startObject, tc.endGroup, tc.forward)
				
				assert.NoError(t, err)
			})
		}
	})
	
	t.Run("subscribe_update_handshake_not_done", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		
		// Don't close handshakeDoneCh - handshake not complete
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		
		err := s.SubscribeUpdate(ctx, 42, 128, 0, 0, 0, 1)
		
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}