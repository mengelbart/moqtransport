package moqtransport

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestMessageRouter(t *testing.T) {
	t.Run("handle_object", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		sink := NewMockSink(ctrl)
		c := NewMockControlMsgSender(ctrl)
		s := newMessageRouter(mc, c)
		s.receiveTracks[0] = sink
		object := &objectMessage{}
		sink.EXPECT().push(object)
		err := s.handleObjectMessage(object)
		assert.NoError(t, err)
	})
	t.Run("handle_client_setup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		c := NewMockControlMsgSender(ctrl)
		s := newMessageRouter(mc, c)
		s.controlMsgSender = c
		csm := &clientSetupMessage{
			SupportedVersions: []version{DRAFT_IETF_MOQ_TRANSPORT_01},
			SetupParameters: map[uint64]parameter{
				roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(IngestionDeliveryRole),
				},
			},
		}
		err := s.handleControlMessage(csm)
		assert.Error(t, err)
		assert.EqualError(t, err, "received unexpected message type on control stream")
	})
	t.Run("handle_subscribe_request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		c := NewMockControlMsgSender(ctrl)
		s := newMessageRouter(mc, c)
		done := make(chan struct{})
		c.EXPECT().send(&subscribeOkMessage{
			TrackNamespace: "namespace",
			TrackName:      "track",
			TrackID:        17,
			Expires:        time.Second,
		}).Do(func(_ message) {
			close(done)
		})
		s.controlMsgSender = c
		go func() {
			err := s.handleControlMessage(&subscribeRequestMessage{
				TrackNamespace: "namespace",
				TrackName:      "track",
				StartGroup:     Location{},
				StartObject:    Location{},
				EndGroup:       Location{},
				EndObject:      Location{},
				Parameters:     map[uint64]parameter{},
			})
			assert.NoError(t, err)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		sub, err := s.readSubscription(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, sub)
		sub.SetExpires(time.Second)
		sub.SetTrackID(17)
		sub.Accept()
		select {
		case <-time.After(time.Second):
			assert.Fail(t, "test timed out")
		case <-done:
		}
	})
	t.Run("handle_announcement", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		c := NewMockControlMsgSender(ctrl)
		s := newMessageRouter(mc, c)
		s.controlMsgSender = c
		done := make(chan struct{})
		c.EXPECT().send(&announceOkMessage{
			TrackNamespace: "namespace",
		}).Do(func(_ message) {
			close(done)
		})
		go func() {
			err := s.handleControlMessage(&announceMessage{
				TrackNamespace:         "namespace",
				TrackRequestParameters: map[uint64]parameter{},
			})
			assert.NoError(t, err)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		a, err := s.readAnnouncement(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, a)
		a.Accept()
		select {
		case <-time.After(time.Second):
			assert.Fail(t, "test timed out")
		case <-done:
		}
	})
	t.Run("subscribe", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		c := NewMockControlMsgSender(ctrl)
		s := newMessageRouter(mc, c)
		s.controlMsgSender = c
		c.EXPECT().send(&subscribeRequestMessage{
			TrackNamespace: "namespace",
			TrackName:      "track",
			StartGroup:     Location{},
			StartObject:    Location{},
			EndGroup:       Location{},
			EndObject:      Location{},
			Parameters: map[uint64]parameter{
				authorizationParameterKey: stringParameter{
					k: authorizationParameterKey,
					v: "auth",
				},
			},
		}).Do(func(_ message) {
			go func() {
				err := s.handleControlMessage(&subscribeOkMessage{
					TrackNamespace: "namespace",
					TrackName:      "track",
					TrackID:        17,
					Expires:        time.Second,
				})
				assert.NoError(t, err)
			}()
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		track, err := s.subscribe(ctx, "namespace", "track", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, track)
	})
	t.Run("announce", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		c := NewMockControlMsgSender(ctrl)
		s := newMessageRouter(mc, c)
		s.controlMsgSender = c
		c.EXPECT().send(&announceMessage{
			TrackNamespace:         "namespace",
			TrackRequestParameters: map[uint64]parameter{},
		}).Do(func(_ message) {
			go func() {
				err := s.handleControlMessage(&announceOkMessage{
					TrackNamespace: "namespace",
				})
				assert.NoError(t, err)
			}()
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := s.announce(ctx, "namespace")
		assert.NoError(t, err)
	})
}
