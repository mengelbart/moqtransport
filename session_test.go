package moqtransport

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func session(conn Connection, ctrlStream controlStreamHandler) *Session {
	return &Session{
		logger:                   slog.Default(),
		closed:                   make(chan struct{}),
		conn:                     conn,
		cms:                      ctrlStream,
		enableDatagrams:          false,
		subscriptionCh:           make(chan *SendSubscription),
		announcementCh:           make(chan *Announcement),
		sendSubscriptionsLock:    sync.RWMutex{},
		sendSubscriptions:        map[uint64]*SendSubscription{},
		receiveSubscriptionsLock: sync.RWMutex{},
		receiveSubscriptions:     map[uint64]*ReceiveSubscription{},
		announcementsLock:        sync.RWMutex{},
		announcements:            map[string]*announcement{},
	}
}

func TestSession(t *testing.T) {
	t.Run("handle_object", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlStreamHandler(ctrl)
		s := *session(mc, csh)
		s.receiveSubscriptions[0] = newReceiveSubscription(0, &s)
		object := &objectMessage{
			SubscribeID:     0,
			TrackAlias:      0,
			GroupID:         0,
			ObjectID:        0,
			ObjectSendOrder: 0,
			ObjectPayload:   []byte{0x0a, 0x0b},
		}
		done := make(chan struct{})
		go func() {
			buf := make([]byte, 1024)
			n, err := s.receiveSubscriptions[0].Read(buf)
			assert.NoError(t, err)
			assert.Equal(t, object.payload(), buf[:n])
			close(done)
		}()
		err := s.handleObjectMessage(object)
		assert.NoError(t, err)
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	t.Run("handle_client_setup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlStreamHandler(ctrl)
		s := session(mc, csh)
		csm := &clientSetupMessage{
			SupportedVersions: []version{CURRENT_VERSION},
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
		csh := NewMockControlStreamHandler(ctrl)
		s := session(mc, csh)
		done := make(chan struct{})
		csh.EXPECT().send(&subscribeOkMessage{
			SubscribeID: 17,
			Expires:     time.Second,
		}).Do(func(_ message) {
			close(done)
		})
		go func() {
			err := s.handleControlMessage(&subscribeMessage{
				SubscribeID:    17,
				TrackAlias:     0,
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
		sub, err := s.ReadSubscription(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, sub)
		sub.SetExpires(time.Second)
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
		csh := NewMockControlStreamHandler(ctrl)
		s := session(mc, csh)
		done := make(chan struct{})
		csh.EXPECT().send(&announceOkMessage{
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
		a, err := s.ReadAnnouncement(ctx)
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
		csh := NewMockControlStreamHandler(ctrl)
		s := session(mc, csh)
		done := make(chan struct{})
		csh.EXPECT().send(&subscribeMessage{
			SubscribeID:    17,
			TrackAlias:     0,
			TrackNamespace: "namespace",
			TrackName:      "track",
			StartGroup:     Location{},
			StartObject:    Location{},
			EndGroup:       Location{},
			EndObject:      Location{},
			Parameters:     map[uint64]parameter{authorizationParameterKey: stringParameter{k: authorizationParameterKey, v: "auth"}},
		}).Do(func(_ message) {
			go func() {
				err := s.handleControlMessage(&subscribeOkMessage{
					SubscribeID: 17,
					Expires:     time.Second,
				})
				assert.NoError(t, err)
				close(done)
			}()
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		track, err := s.Subscribe(ctx, 17, 0, "namespace", "track", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, track)
		<-done
	})
	t.Run("announce", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlStreamHandler(ctrl)
		s := session(mc, csh)
		csh.EXPECT().send(&announceMessage{
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
		err := s.Announce(ctx, "namespace")
		assert.NoError(t, err)
	})
}
