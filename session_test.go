package moqtransport

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

func newSession(cms controlMessageSender, handler messageHandler, protocol Protocol, perspective Perspective) *Session {
	return &Session{
		logger:                                   defaultLogger,
		destroyOnce:                              sync.Once{},
		handshakeDoneCh:                          make(chan struct{}),
		controlMessageSender:                     cms,
		pvh:                                      nil,
		handler:                                  handler,
		version:                                  0,
		protocol:                                 protocol,
		perspective:                              perspective,
		localRole:                                RolePubSub,
		remoteRole:                               0,
		path:                                     "/path",
		outgoingAnnouncements:                    newAnnouncementMap(),
		incomingAnnouncements:                    newAnnouncementMap(),
		pendingOutgointAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		pendingIncomingAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		highestSubscribesBlocked:                 atomic.Uint64{},
		outgoingSubscriptions:                    newSubscriptionMap(0),
		incomingSubscriptions:                    newSubscriptionMap(100),
	}
}

func TestSession(t *testing.T) {
	t.Run("sends_client_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)

		cms.EXPECT().QueueControlMessage(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.RoleParameterKey: wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
				wire.PathParameterKey: wire.StringParameter{
					Type:  wire.PathParameterKey,
					Value: "/path",
				},
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		})
		err := s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_client_setup_wt", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)

		s := newSession(cms, mh, ProtocolWebTransport, PerspectiveClient)

		cms.EXPECT().QueueControlMessage(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.RoleParameterKey: wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		})
		err := s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_server_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)

		cms.EXPECT().QueueControlMessage(&wire.ServerSetupMessage{
			SelectedVersion: wire.CurrentVersion,
			SetupParameters: map[uint64]wire.Parameter{
				wire.RoleParameterKey: wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(RolePubSub),
				},
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		})

		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.RoleParameterKey: wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
				wire.PathParameterKey: wire.StringParameter{
					Type:  wire.PathParameterKey,
					Value: "/path",
				},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("sends_server_setup_wt", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)

		s := newSession(cms, mh, ProtocolWebTransport, PerspectiveServer)

		cms.EXPECT().QueueControlMessage(&wire.ServerSetupMessage{
			SelectedVersion: wire.CurrentVersion,
			SetupParameters: map[uint64]wire.Parameter{
				wire.RoleParameterKey: wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		})

		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.RoleParameterKey: wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("rejects_quic_client_without_path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.RoleParameterKey: wire.VarintParameter{
					Type:  wire.RoleParameterKey,
					Value: uint64(wire.RolePubSub),
				},
			},
		})
		assert.Error(t, err)
	})

	t.Run("rejects_quic_client_without_role", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.PathParameterKey: wire.StringParameter{
					Type:  wire.PathParameterKey,
					Value: "/path",
				},
			},
		})
		assert.Error(t, err)
	})

	t.Run("rejects_wt_client_without_role", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)
		s := newSession(cms, mh, ProtocolWebTransport, PerspectiveServer)
		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.PathParameterKey: wire.StringParameter{
					Type:  wire.PathParameterKey,
					Value: "/path",
				},
			},
		})
		assert.Error(t, err)
	})

	t.Run("rejects_subscribe_on_max_subscribe_id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)

		cms.EXPECT().QueueControlMessage(&wire.SubscribesBlocked{
			MaximumSubscribeID: 1,
		}).Times(1)

		s.outgoingSubscriptions.maxSubscribeID = 1
		close(s.handshakeDoneCh)
		rt, err := s.Subscribe(context.Background(), 2, 0, []string{"namespace"}, "track", "auth")
		assert.Error(t, err)
		assert.Nil(t, rt)

		rt, err = s.Subscribe(context.Background(), 2, 0, []string{"namespace"}, "track", "auth")
		assert.Error(t, err)
		assert.Nil(t, rt)
	})

	t.Run("sends_subscribe", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		s.outgoingSubscriptions.maxSubscribeID = 1
		cms.EXPECT().QueueControlMessage(&wire.SubscribeMessage{
			SubscribeID:        0,
			TrackAlias:         0,
			TrackNamespace:     []string{"namespace"},
			TrackName:          []byte("track"),
			SubscriberPriority: 0,
			GroupOrder:         0,
			FilterType:         0,
			StartGroup:         0,
			StartObject:        0,
			EndGroup:           0,
			EndObject:          0,
			Parameters: map[uint64]wire.Parameter{
				2: &wire.StringParameter{
					Type:  2,
					Value: "auth",
				},
			},
		}).Do(func(_ wire.ControlMessage) {
			err := s.receive(&wire.SubscribeOkMessage{
				SubscribeID:     0,
				Expires:         0,
				GroupOrder:      0,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      map[uint64]wire.Parameter{},
			})
			assert.NoError(t, err)
		})
		rt, err := s.Subscribe(context.Background(), 0, 0, []string{"namespace"}, "track", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("sends_subscribe_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mh := NewMockMessageHandler(ctrl)
		cms := NewMockControlMessageSender(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		mh.EXPECT().handle(&Message{
			Method:        MessageSubscribe,
			SubscribeID:   0,
			TrackAlias:    0,
			Namespace:     []string{"namespace"},
			Track:         "track",
			Authorization: "",
			Status:        0,
			LastGroupID:   0,
			LastObjectID:  0,
			NewSessionURI: "",
			ErrorCode:     0,
			ReasonPhrase:  "",
		}).Do(func(_ *Message) {
			err := s.acceptSubscription(0, nil)
			assert.NoError(t, err)
		})
		cms.EXPECT().QueueControlMessage(&wire.SubscribeOkMessage{
			SubscribeID:     0,
			Expires:         0,
			GroupOrder:      0,
			ContentExists:   false,
			LargestGroupID:  0,
			LargestObjectID: 0,
			Parameters:      map[uint64]wire.Parameter{},
		})
		err := s.receive(&wire.SubscribeMessage{
			SubscribeID:        0,
			TrackAlias:         0,
			TrackNamespace:     []string{"namespace"},
			TrackName:          []byte("track"),
			SubscriberPriority: 0,
			GroupOrder:         0,
			FilterType:         0,
			StartGroup:         0,
			StartObject:        0,
			EndGroup:           0,
			EndObject:          0,
			Parameters:         map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
	})

	t.Run("sends_subscribe_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mh := NewMockMessageHandler(ctrl)
		cms := NewMockControlMessageSender(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		mh.EXPECT().handle(
			&Message{
				Method:        MessageSubscribe,
				Namespace:     []string{},
				Track:         "",
				Authorization: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
		).DoAndReturn(func(m *Message) {
			assert.NoError(t, s.rejectSubscription(m.SubscribeID, ErrorCodeSubscribeTrackDoesNotExist, "track not found"))
		})
		cms.EXPECT().QueueControlMessage(&wire.SubscribeErrorMessage{
			SubscribeID:  0,
			ErrorCode:    ErrorCodeSubscribeTrackDoesNotExist,
			ReasonPhrase: "track not found",
			TrackAlias:   0,
		})
		err := s.receive(&wire.SubscribeMessage{
			SubscribeID:        0,
			TrackAlias:         0,
			TrackNamespace:     []string{},
			TrackName:          []byte{},
			SubscriberPriority: 0,
			GroupOrder:         0,
			FilterType:         0,
			StartGroup:         0,
			StartObject:        0,
			EndGroup:           0,
			EndObject:          0,
			Parameters:         map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
	})

	t.Run("sends_announce", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		cms.EXPECT().QueueControlMessage(&wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		}).Do(func(_ wire.ControlMessage) {
			err := s.receive(&wire.AnnounceOkMessage{
				TrackNamespace: []string{"namespace"},
			})
			assert.NoError(t, err)
		})
		err := s.Announce(context.Background(), []string{"namespace"})
		assert.NoError(t, err)
	})

	t.Run("sends_announce_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSender(ctrl)
		mh := NewMockMessageHandler(ctrl)
		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		mh.EXPECT().handle(&Message{
			Method:    MessageAnnounce,
			Namespace: []string{"namespace"},
		}).DoAndReturn(func(req *Message) {
			assert.NoError(t, s.acceptAnnouncement(req.Namespace))
		})
		cms.EXPECT().QueueControlMessage(&wire.AnnounceOkMessage{
			TrackNamespace: []string{"namespace"},
		})
		err := s.receive(&wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
	})
}
