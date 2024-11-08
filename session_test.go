package moqtransport

import (
	"context"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
)

func assertFirstControlMessageEqual(t *testing.T, s *Session, expect wire.ControlMessage) {
	var msg wire.ControlMessage
	select {
	case msg = <-s.SendControlMessages():
	default:
		assert.Fail(t, "session did not send control message")
	}
	assert.Equal(t, expect, msg)
}

func assertFirstSubscriptionEqual(t *testing.T, s *Session, expect Subscription) Subscription {
	var sub Subscription
	select {
	case sub = <-s.IncomingSubscriptions():
	default:
		assert.Fail(t, "session did not receive subscription")
	}
	assert.Equal(t, expect, sub)
	return sub
}

func assertFirstAnnouncementEqual(t *testing.T, s *Session, expect Announcement) Announcement {
	var an Announcement
	select {
	case an = <-s.IncomingAnnouncements():
	default:
		assert.Fail(t, "session did not receive announcement")
	}
	assert.Equal(t, expect, an)
	return an
}

func TestSession(t *testing.T) {
	t.Run("sends_client_setup_quic", func(t *testing.T) {
		s, err := NewSession(
			false,
			true,
			RoleParameterOption(RolePubSub),
			PathParameterOption("/path"),
			MaxSubscribeIDOption(100),
		)
		assert.NoError(t, err)
		assertFirstControlMessageEqual(t, s, &wire.ClientSetupMessage{
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
	})

	t.Run("sends_client_setup_wt", func(t *testing.T) {
		s, err := NewSession(
			false,
			false,
			RoleParameterOption(RolePubSub),
			MaxSubscribeIDOption(100),
		)
		assert.NoError(t, err)
		assertFirstControlMessageEqual(t, s, &wire.ClientSetupMessage{
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
	})

	t.Run("sends_server_setup_quic", func(t *testing.T) {
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		err = s.OnControlMessage(&wire.ClientSetupMessage{
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
		assertFirstControlMessageEqual(t, s, &wire.ServerSetupMessage{
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
	})

	t.Run("sends_server_setup_wt", func(t *testing.T) {
		s, err := NewSession(true, false)
		assert.NoError(t, err)
		err = s.OnControlMessage(&wire.ClientSetupMessage{
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

		assertFirstControlMessageEqual(t, s, &wire.ServerSetupMessage{
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
	})

	t.Run("rejects_quic_client_without_path", func(t *testing.T) {
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		err = s.OnControlMessage(&wire.ClientSetupMessage{
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
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		err = s.OnControlMessage(&wire.ClientSetupMessage{
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
		s, err := NewSession(true, false)
		assert.NoError(t, err)
		err = s.OnControlMessage(&wire.ClientSetupMessage{
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
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		s.setupDone = true
		err = s.Subscribe(context.Background(), Subscription{
			ID:            0,
			TrackAlias:    0,
			Namespace:     []string{"namespace"},
			Trackname:     "track",
			Authorization: "",
			Expires:       0,
			GroupOrder:    0,
			ContentExists: false,
			publisher:     &Publisher{},
			remoteTrack:   &RemoteTrack{},
			response:      make(chan subscriptionResponse),
		})
		assert.Error(t, err)
	})

	t.Run("sends_subscribe", func(t *testing.T) {
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		s.setupDone = true
		s.remoteMaxSubscribeID = 1
		err = s.Subscribe(context.Background(), Subscription{
			ID:            0,
			TrackAlias:    0,
			Namespace:     nil,
			Trackname:     "track",
			Authorization: "",
			Expires:       0,
			GroupOrder:    0,
			ContentExists: false,
			publisher:     &Publisher{},
			remoteTrack:   &RemoteTrack{},
			response:      make(chan subscriptionResponse),
		})
		assert.NoError(t, err)
		assertFirstControlMessageEqual(t, s, &wire.SubscribeMessage{
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
	})

	t.Run("sends_subscribe_ok", func(t *testing.T) {
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		s.setupDone = true
		err = s.OnControlMessage(&wire.SubscribeMessage{
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
		sub := assertFirstSubscriptionEqual(t, s, Subscription{
			ID:            0,
			TrackAlias:    0,
			Namespace:     []string{},
			Trackname:     "",
			Authorization: "",
		})
		err = s.AcceptSubscription(sub)
		assert.NoError(t, err)
		assertFirstControlMessageEqual(t, s, &wire.SubscribeOkMessage{
			SubscribeID:     0,
			Expires:         0,
			GroupOrder:      0,
			ContentExists:   false,
			LargestGroupID:  0,
			LargestObjectID: 0,
			Parameters:      map[uint64]wire.Parameter{},
		})
	})

	t.Run("sends_subscribe_error", func(t *testing.T) {
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		s.setupDone = true
		err = s.OnControlMessage(&wire.SubscribeMessage{
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
		sub := assertFirstSubscriptionEqual(t, s, Subscription{
			ID:            0,
			TrackAlias:    0,
			Namespace:     []string{},
			Trackname:     "",
			Authorization: "",
		})
		err = s.RejectSubscription(sub, SubscribeErrorTrackDoesNotExist, "track not found")
		assert.NoError(t, err)
		assertFirstControlMessageEqual(t, s, &wire.SubscribeErrorMessage{
			SubscribeID:  0,
			ErrorCode:    SubscribeErrorTrackDoesNotExist,
			ReasonPhrase: "track not found",
			TrackAlias:   0,
		})
	})

	t.Run("sends_announce", func(t *testing.T) {
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		s.setupDone = true
		err = s.Announce([]string{"namespace"})
		assert.NoError(t, err)
		assertFirstControlMessageEqual(t, s, &wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		})
	})

	t.Run("sends_announce_ok", func(t *testing.T) {
		s, err := NewSession(true, true)
		assert.NoError(t, err)
		s.setupDone = true
		err = s.OnControlMessage(&wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
		announcement := assertFirstAnnouncementEqual(t, s, Announcement{
			namespace:  []string{"namespace"},
			parameters: map[uint64]wire.Parameter{},
		})
		err = s.AcceptAnnouncement(announcement)
		assert.NoError(t, err)
		assertFirstControlMessageEqual(t, s, &wire.AnnounceOkMessage{
			TrackNamespace: []string{"namespace"},
		})
	})
}
