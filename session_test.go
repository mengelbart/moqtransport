package moqtransport

import (
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSession(t *testing.T) {
	t.Run("sends_client_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcb := NewMockSessionCallbacks(ctrl)
		mcb.EXPECT().queueControlMessage(&wire.ClientSetupMessage{
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
		s, err := newSession(
			mcb,
			false,
			true,
			roleParameterOption(RolePubSub),
			pathParameterOption("/path"),
			maxSubscribeIDOption(100),
		)
		assert.NoError(t, err)
		err = s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_client_setup_wt", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcb := NewMockSessionCallbacks(ctrl)
		mcb.EXPECT().queueControlMessage(&wire.ClientSetupMessage{
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
		s, err := newSession(
			mcb,
			false,
			false,
			roleParameterOption(RolePubSub),
			maxSubscribeIDOption(100),
		)
		assert.NoError(t, err)
		err = s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_server_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		mcb.EXPECT().queueControlMessage(&wire.ServerSetupMessage{
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
		err = s.onControlMessage(&wire.ClientSetupMessage{
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
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, false)
		assert.NoError(t, err)
		mcb.EXPECT().queueControlMessage(&wire.ServerSetupMessage{
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
		err = s.onControlMessage(&wire.ClientSetupMessage{
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
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		err = s.onControlMessage(&wire.ClientSetupMessage{
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
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		err = s.onControlMessage(&wire.ClientSetupMessage{
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
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, false)
		assert.NoError(t, err)
		err = s.onControlMessage(&wire.ClientSetupMessage{
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
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		s.setupDone = true
		err = s.subscribe(&subscription{
			ID:            0,
			TrackAlias:    0,
			Namespace:     []string{"namespace"},
			Trackname:     "track",
			Authorization: "",
			Expires:       0,
			GroupOrder:    0,
			ContentExists: false,
			remoteTrack:   &RemoteTrack{},
			response:      make(chan subscriptionResponse),
		})
		assert.Error(t, err)
	})

	t.Run("sends_subscribe", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		s.setupDone = true
		s.outgoingSubscriptions.maxSubscribeID = 1
		mcb.EXPECT().queueControlMessage(&wire.SubscribeMessage{
			SubscribeID:        0,
			TrackAlias:         0,
			TrackNamespace:     nil,
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
		err = s.subscribe(&subscription{
			ID:            0,
			TrackAlias:    0,
			Namespace:     nil,
			Trackname:     "track",
			Authorization: "",
			Expires:       0,
			GroupOrder:    0,
			ContentExists: false,
			remoteTrack:   &RemoteTrack{},
			response:      make(chan subscriptionResponse),
		})
		assert.NoError(t, err)
	})

	t.Run("sends_subscribe_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		s.setupDone = true
		mcb.EXPECT().onMessage(
			&Message{
				Method:        MethodSubscribe,
				Namespace:     []string{},
				Track:         "",
				Authorization: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
		).Do(func(m *Message) {
			assert.NoError(t, s.acceptSubscription(m.SubscribeID, nil))
		})
		mcb.EXPECT().queueControlMessage(&wire.SubscribeOkMessage{
			SubscribeID:     0,
			Expires:         0,
			GroupOrder:      0,
			ContentExists:   false,
			LargestGroupID:  0,
			LargestObjectID: 0,
			Parameters:      map[uint64]wire.Parameter{},
		})
		err = s.onControlMessage(&wire.SubscribeMessage{
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

	t.Run("sends_subscribe_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		s.setupDone = true
		mcb.EXPECT().onMessage(
			&Message{
				Method:        MethodSubscribe,
				Namespace:     []string{},
				Track:         "",
				Authorization: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
		).DoAndReturn(func(m *Message) {
			assert.NoError(t, s.rejectSubscription(m.SubscribeID, SubscribeErrorTrackDoesNotExist, "track not found"))
		})
		mcb.EXPECT().queueControlMessage(&wire.SubscribeErrorMessage{
			SubscribeID:  0,
			ErrorCode:    SubscribeErrorTrackDoesNotExist,
			ReasonPhrase: "track not found",
			TrackAlias:   0,
		})
		err = s.onControlMessage(&wire.SubscribeMessage{
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
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		s.setupDone = true
		mcb.EXPECT().queueControlMessage(&wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		})
		err = s.announce(&announcement{
			Namespace:  []string{"namespace"},
			parameters: map[uint64]wire.Parameter{},
			response:   make(chan announcementResponse),
		})
		assert.NoError(t, err)
	})

	t.Run("sends_announce_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mcb := NewMockSessionCallbacks(ctrl)
		s, err := newSession(mcb, true, true)
		assert.NoError(t, err)
		s.setupDone = true
		mcb.EXPECT().onMessage(&Message{
			Method:    MethodAnnounce,
			Namespace: []string{"namespace"},
		}).DoAndReturn(func(req *Message) {
			assert.NoError(t, s.acceptAnnouncement(req.Namespace))
		})
		mcb.EXPECT().queueControlMessage(&wire.AnnounceOkMessage{
			TrackNamespace: []string{"namespace"},
		})
		err = s.onControlMessage(&wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
	})
}
