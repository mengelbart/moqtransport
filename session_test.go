package moqtransport

import (
	"context"
	"testing"
	"time"

	"github.com/mengelbart/moqtransport/internal/wire"
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
		SubscriptionHandler: nil,
		handshakeDone:       false,
		controlStream:       nil,
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
		err := s.si.receiveSubscriptions.add(0, newRemoteTrack(0, s))
		assert.NoError(t, err)
		object := Object{
			GroupID:              0,
			ObjectID:             0,
			ObjectSendOrder:      0,
			ForwardingPreference: 0,
			Payload:              []byte{0x0a, 0x0b},
		}
		go func() {
			sub, ok := s.si.receiveSubscriptions.get(0)
			assert.True(t, ok)
			o, err1 := sub.ReadObject(context.Background())
			assert.NoError(t, err1)
			assert.Equal(t, object, o)
			close(done)
		}()
		sub, ok := s.si.receiveSubscriptions.get(0)
		assert.True(t, ok)
		sub.push(Object{
			GroupID:              object.GroupID,
			ObjectID:             object.ObjectID,
			ObjectSendOrder:      0,
			ForwardingPreference: ObjectForwardingPreferenceDatagram,
			Payload:              object.Payload,
		})
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
		csm := &wire.ClientSetupMessage{
			SupportedVersions: []wire.Version{wire.CurrentVersion},
			SetupParameters: wire.Parameters{
				wire.RoleParameterKey: &wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
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
		csh.EXPECT().enqueue(&wire.SubscribeOkMessage{
			SubscribeID:   17,
			Expires:       0,
			ContentExists: false,
			FinalGroup:    0,
			FinalObject:   0,
		}).Do(func(_ wire.Message) {
			close(done)
		})
		track := NewLocalTrack("namespace", "track")
		defer track.Close()
		err := s.AddLocalTrack(track)
		assert.NoError(t, err)
		err = s.handleControlMessage(&wire.ClientSetupMessage{
			SupportedVersions: []wire.Version{wire.CurrentVersion},
			SetupParameters: wire.Parameters{
				wire.RoleParameterKey: &wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
			},
		})
		assert.NoError(t, err)
		err = s.handleControlMessage(&wire.SubscribeMessage{
			SubscribeID:    17,
			TrackAlias:     0,
			TrackNamespace: "namespace",
			TrackName:      "track",
			FilterType:     0,
			StartGroup:     0,
			StartObject:    0,
			EndGroup:       0,
			EndObject:      0,
			Parameters:     wire.Parameters{},
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
		s := session(mc, csh, AnnouncementHandlerFunc(func(s *Session, a *Announcement, arw AnnouncementResponseWriter) {
			assert.NotNil(t, a)
			arw.Accept()
		}))
		done := make(chan struct{})
		csh.EXPECT().enqueue(gomock.Any()).Times(1) // setup message
		csh.EXPECT().enqueue(&wire.AnnounceOkMessage{
			TrackNamespace: "namespace",
		}).Do(func(_ wire.Message) {
			close(done)
		})
		err := s.handleControlMessage(&wire.ClientSetupMessage{
			SupportedVersions: []wire.Version{wire.CurrentVersion},
			SetupParameters: wire.Parameters{
				wire.RoleParameterKey: &wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
			},
		})
		assert.NoError(t, err)
		err = s.handleControlMessage(&wire.AnnounceMessage{
			TrackNamespace: "namespace",
			Parameters:     wire.Parameters{},
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
		csh.EXPECT().enqueue(&wire.SubscribeMessage{
			SubscribeID:    17,
			TrackAlias:     0,
			TrackNamespace: "namespace",
			TrackName:      "track",
			FilterType:     0,
			StartGroup:     0,
			StartObject:    0,
			EndGroup:       0,
			EndObject:      0,
			Parameters:     wire.Parameters{wire.AuthorizationParameterKey: &wire.StringParameter{Type: wire.AuthorizationParameterKey, Value: "auth"}},
		}).Do(func(_ wire.Message) {
			go func() {
				err := s.handleControlMessage(&wire.SubscribeOkMessage{
					SubscribeID: 17,
					Expires:     time.Second,
				})
				assert.NoError(t, err)
				close(done)
			}()
		})
		err := s.handleControlMessage(&wire.ClientSetupMessage{
			SupportedVersions: []wire.Version{wire.CurrentVersion},
			SetupParameters: wire.Parameters{
				wire.RoleParameterKey: &wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
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
		csh.EXPECT().enqueue(&wire.AnnounceMessage{
			TrackNamespace: "namespace",
			Parameters:     wire.Parameters{},
		}).Do(func(_ wire.Message) {
			go func() {
				err := s.handleControlMessage(&wire.AnnounceOkMessage{
					TrackNamespace: "namespace",
				})
				assert.NoError(t, err)
			}()
		})
		err := s.handleControlMessage(&wire.ClientSetupMessage{
			SupportedVersions: []wire.Version{wire.CurrentVersion},
			SetupParameters: wire.Parameters{
				wire.RoleParameterKey: &wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
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
