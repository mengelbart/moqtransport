package moqtransport

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func session(conn Connection, ctrl controlMessageSender, h AnnouncementHandler) *Session {
	s := &Session{
		Conn:                conn,
		EnableDatagrams:     false,
		LocalRole:           0,
		RemoteRole:          0,
		AnnouncementHandler: h,
		HandshakeDone:       false,
		isClient:            false,
		si:                  newSessionInternals("SERVER"),
	}
	s.storeControlStream(ctrl)
	return s
}

func TestSession(t *testing.T) {
	t.Run("handle_object", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		done := make(chan struct{})
		s := session(mc, nil, nil)
		err := s.si.receiveSubscriptions.add(0, newReceiveSubscription(0, s))
		assert.NoError(t, err)
		object := &objectMessage{
			SubscribeID:     0,
			TrackAlias:      0,
			GroupID:         0,
			ObjectID:        0,
			ObjectSendOrder: 0,
			ObjectPayload:   []byte{0x0a, 0x0b},
		}
		go func() {
			buf := make([]byte, 1024)
			sub, ok := s.si.receiveSubscriptions.get(0)
			assert.True(t, ok)
			n, err1 := sub.Read(buf)
			assert.NoError(t, err1)
			assert.Equal(t, object.payload(), buf[:n])
			close(done)
		}()
		err = s.handleObjectMessage(object)
		assert.NoError(t, err)
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	t.Run("handle_client_setup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlMessageSender(ctrl)
		csh.EXPECT().enqueue(gomock.Any()).AnyTimes()
		done := make(chan struct{})
		s := session(mc, csh, nil)
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
		assert.NoError(t, err)
		close(done)
	})
	t.Run("handle_subscribe_request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlMessageSender(ctrl)
		s := session(mc, csh, nil)
		done := make(chan struct{})
		csh.EXPECT().enqueue(gomock.Any()).Times(1) // Setup message
		csh.EXPECT().enqueue(&subscribeOkMessage{
			SubscribeID:   17,
			Expires:       0,
			ContentExists: false,
			FinalGroup:    0,
			FinalObject:   0,
		}).Do(func(_ message) {
			close(done)
		})
		track := NewLocalTrack(0, "namespace", "track")
		defer track.Close()
		err := s.AddLocalTrack(track)
		assert.NoError(t, err)
		err = s.handleControlMessage(&clientSetupMessage{
			SupportedVersions: []version{CURRENT_VERSION},
			SetupParameters: map[uint64]parameter{
				roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(IngestionDeliveryRole),
				},
			},
		})
		assert.NoError(t, err)
		err = s.handleControlMessage(&subscribeMessage{
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
		select {
		case <-time.After(time.Second):
			assert.Fail(t, "test timed out")
		case <-done:
		}
	})
	t.Run("handle_announcement", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlMessageSender(ctrl)
		s := session(mc, csh, AnnouncementHandlerFunc(func(a *Announcement, arw AnnouncementResponseWriter) {
			assert.NotNil(t, a)
			arw.Accept()
		}))
		done := make(chan struct{})
		csh.EXPECT().enqueue(gomock.Any()).Times(1) // setup message
		csh.EXPECT().enqueue(&announceOkMessage{
			TrackNamespace: "namespace",
		}).Do(func(_ message) {
			close(done)
		})
		err := s.handleControlMessage(&clientSetupMessage{
			SupportedVersions: []version{CURRENT_VERSION},
			SetupParameters: map[uint64]parameter{
				roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(IngestionDeliveryRole),
				},
			},
		})
		assert.NoError(t, err)
		err = s.handleControlMessage(&announceMessage{
			TrackNamespace:         "namespace",
			TrackRequestParameters: map[uint64]parameter{},
		})
		assert.NoError(t, err)
		select {
		case <-time.After(time.Second):
			assert.Fail(t, "test timed out")
		case <-done:
		}
	})
	t.Run("subscribe", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlMessageSender(ctrl)
		s := session(mc, csh, nil)
		done := make(chan struct{})
		csh.EXPECT().enqueue(gomock.Any()).Times(1)
		csh.EXPECT().enqueue(&subscribeMessage{
			SubscribeID:    17,
			TrackAlias:     0,
			TrackNamespace: "namespace",
			TrackName:      "track",
			StartGroup:     Location{LocationModeAbsolute, 0x00},
			StartObject:    Location{LocationModeAbsolute, 0x00},
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
		err := s.handleControlMessage(&clientSetupMessage{
			SupportedVersions: []version{CURRENT_VERSION},
			SetupParameters: map[uint64]parameter{
				roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(IngestionDeliveryRole),
				},
			},
		})
		assert.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		track, err := s.Subscribe(ctx, 17, 0, "namespace", "track", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, track)
		select {
		case <-time.After(time.Second):
			assert.Fail(t, "test timed out")
		case <-done:
		}
	})
	t.Run("announce", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mc := NewMockConnection(ctrl)
		csh := NewMockControlMessageSender(ctrl)
		s := session(mc, csh, nil)
		csh.EXPECT().enqueue(gomock.Any()).Times(1)
		csh.EXPECT().enqueue(&announceMessage{
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
		err := s.handleControlMessage(&clientSetupMessage{
			SupportedVersions: []version{CURRENT_VERSION},
			SetupParameters: map[uint64]parameter{
				roleParameterKey: varintParameter{
					k: roleParameterKey,
					v: uint64(IngestionDeliveryRole),
				},
			},
		})
		assert.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = s.Announce(ctx, "namespace")
		assert.NoError(t, err)
	})
}
