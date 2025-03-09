package moqtransport

import (
	"context"
	"io"
	"sync/atomic"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
)

func newSession(queue controlMessageQueue[wire.ControlMessage], handler controlMessageQueue[*Message], protocol Protocol, perspective Perspective) *Session {
	return &Session{
		logger:                                   defaultLogger,
		handshakeDoneCh:                          make(chan struct{}),
		ctrlMsgSendQueue:                         queue,
		ctrlMsgReceiveQueue:                      handler,
		version:                                  0,
		Protocol:                                 protocol,
		Perspective:                              perspective,
		path:                                     "/path",
		MaxSubscribeID:                           100,
		outgoingAnnouncements:                    newAnnouncementMap(),
		incomingAnnouncements:                    newAnnouncementMap(),
		pendingOutgointAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		pendingIncomingAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		highestSubscribesBlocked:                 atomic.Uint64{},
		remoteTracks:                             newRemoteTrackMap(0),
		localTracks:                              newLocalTrackMap(),
		outgoingTrackStatusRequests:              newTrackStatusRequestMap(),
	}
}

func TestSession(t *testing.T) {
	t.Run("sends_client_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		cms.EXPECT().enqueue(context.Background(),
			&wire.ClientSetupMessage{
				SupportedVersions: wire.SupportedVersions,
				SetupParameters: map[uint64]wire.Parameter{
					wire.PathParameterKey: wire.StringParameter{
						Type:  wire.PathParameterKey,
						Value: "/path",
					},
					wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
						Type:  wire.MaxSubscribeIDParameterKey,
						Value: 100,
					},
				},
			},
		)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		err := s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_client_setup_wt", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		cms.EXPECT().enqueue(context.Background(), &wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		},
		)

		s := newSession(cms, mh, ProtocolWebTransport, PerspectiveClient)
		err := s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_server_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)

		cms.EXPECT().enqueue(context.Background(), &wire.ServerSetupMessage{
			SelectedVersion: wire.CurrentVersion,
			SetupParameters: map[uint64]wire.Parameter{
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		})

		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
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
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolWebTransport, PerspectiveServer)

		cms.EXPECT().enqueue(context.Background(), &wire.ServerSetupMessage{
			SelectedVersion: wire.CurrentVersion,
			SetupParameters: map[uint64]wire.Parameter{
				wire.MaxSubscribeIDParameterKey: wire.VarintParameter{
					Type:  wire.MaxSubscribeIDParameterKey,
					Value: 100,
				},
			},
		})

		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: map[uint64]wire.Parameter{
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
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveServer)
		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters:   map[uint64]wire.Parameter{},
		})
		assert.Error(t, err)
	})

	t.Run("rejects_subscribe_on_max_subscribe_id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)

		cms.EXPECT().enqueue(context.Background(), &wire.SubscribeMessage{
			SubscribeID:        0,
			TrackAlias:         0,
			TrackNamespace:     wire.Tuple{"namespace"},
			TrackName:          []byte("track1"),
			SubscriberPriority: 0,
			GroupOrder:         0,
			FilterType:         0,
			StartGroup:         0,
			StartObject:        0,
			EndGroup:           0,
			Parameters: wire.Parameters{
				wire.AuthorizationParameterKey: &wire.StringParameter{
					Type:  wire.AuthorizationParameterKey,
					Value: "auth",
				},
			},
		}).DoAndReturn(func(_ context.Context, _ wire.ControlMessage) error {
			err := s.onSubscribeOk(&wire.SubscribeOkMessage{
				SubscribeID:     0,
				Expires:         0,
				GroupOrder:      0,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      wire.Parameters{},
			})
			assert.NoError(t, err)
			return nil
		}).Times(1)
		cms.EXPECT().enqueue(context.Background(), &wire.SubscribesBlockedMessage{
			MaximumSubscribeID: 1,
		}).Times(1)

		s.remoteTracks.maxSubscribeID = 1
		close(s.handshakeDoneCh)
		rt, err := s.Subscribe(context.Background(), []string{"namespace"}, "track1", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, rt)

		rt, err = s.Subscribe(context.Background(), []string{"namespace"}, "track2", "auth")
		assert.Error(t, err)
		assert.Nil(t, rt)
	})

	t.Run("sends_subscribe", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		s.remoteTracks.maxSubscribeID = 1
		cms.EXPECT().enqueue(context.Background(), &wire.SubscribeMessage{
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
			Parameters: map[uint64]wire.Parameter{
				2: &wire.StringParameter{
					Type:  2,
					Value: "auth",
				},
			},
		}).DoAndReturn(func(_ context.Context, _ wire.ControlMessage) error {
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
			return nil
		})
		rt, err := s.Subscribe(context.Background(), []string{"namespace"}, "track", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("sends_subscribe_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		mh.EXPECT().enqueue(context.Background(), &Message{
			Method:        MessageSubscribe,
			SubscribeID:   0,
			TrackAlias:    0,
			Namespace:     []string{"namespace"},
			Track:         "track",
			Authorization: "",
			NewSessionURI: "",
			ErrorCode:     0,
			ReasonPhrase:  "",
		}).Do(func(_ context.Context, _ *Message) error {
			assert.NoError(t, s.addLocalTrack(&localTrack{}))
			assert.NoError(t, s.acceptSubscription(0))
			return nil
		})
		cms.EXPECT().enqueue(context.Background(), &wire.SubscribeOkMessage{
			SubscribeID:     0,
			Expires:         0,
			GroupOrder:      1,
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
			Parameters:         map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
	})

	t.Run("sends_subscribe_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		mh.EXPECT().enqueue(
			context.Background(),
			&Message{
				Method:        MessageSubscribe,
				Namespace:     []string{},
				Track:         "",
				Authorization: "",
				ErrorCode:     0,
				ReasonPhrase:  "",
			},
		).DoAndReturn(func(_ context.Context, m *Message) error {
			assert.NoError(t, s.addLocalTrack(&localTrack{}))
			assert.NoError(t, s.rejectSubscription(m.SubscribeID, ErrorCodeSubscribeTrackDoesNotExist, "track not found"))
			return nil
		})
		cms.EXPECT().enqueue(context.Background(), &wire.SubscribeErrorMessage{
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
			Parameters:         map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
	})

	t.Run("sends_announce", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		cms.EXPECT().enqueue(context.Background(), &wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		}).DoAndReturn(func(_ context.Context, _ wire.ControlMessage) error {
			err := s.receive(&wire.AnnounceOkMessage{
				TrackNamespace: []string{"namespace"},
			})
			assert.NoError(t, err)
			return nil
		})
		err := s.Announce(context.Background(), []string{"namespace"})
		assert.NoError(t, err)
	})

	t.Run("sends_announce_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		close(s.handshakeDoneCh)
		mh.EXPECT().enqueue(context.Background(), &Message{
			Method:    MessageAnnounce,
			Namespace: []string{"namespace"},
		}).DoAndReturn(func(_ context.Context, req *Message) error {
			assert.NoError(t, s.acceptAnnouncement(req.Namespace))
			return nil
		})
		cms.EXPECT().enqueue(context.Background(), &wire.AnnounceOkMessage{
			TrackNamespace: []string{"namespace"},
		})
		err := s.receive(&wire.AnnounceMessage{
			TrackNamespace: []string{"namespace"},
			Parameters:     map[uint64]wire.Parameter{},
		})
		assert.NoError(t, err)
	})

	t.Run("receives_objects_before_susbcribe_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cms := NewMockControlMessageSendQueue[wire.ControlMessage](ctrl)
		mh := NewMockControlMessageRecvQueue[*Message](ctrl)
		mp := NewMockObjectMessageParser(ctrl)

		mp.EXPECT().Type().Return(wire.StreamTypeSubgroup).AnyTimes()
		mp.EXPECT().SubscribeID().Return(uint64(0), nil).AnyTimes()
		mp.EXPECT().TrackAlias().Return(uint64(0), nil).AnyTimes()

		mp.EXPECT().Messages().Return(func(yield func(*wire.ObjectMessage, error) bool) {
			if !yield(&wire.ObjectMessage{
				TrackAlias:             2,
				GroupID:                0,
				SubgroupID:             0,
				ObjectID:               0,
				PublisherPriority:      0,
				ObjectHeaderExtensions: []wire.ObjectHeaderExtension{},
				ObjectStatus:           0,
				ObjectPayload:          []byte{},
			}, nil) {
				assert.Fail(t, "yield returned false")
				return
			}
			yield(nil, io.EOF)
		})

		s := newSession(cms, mh, ProtocolQUIC, PerspectiveClient)
		err := s.onMaxSubscribeID(&wire.MaxSubscribeIDMessage{
			SubscribeID: 100,
		})
		assert.NoError(t, err)
		close(s.handshakeDoneCh)
		cms.EXPECT().enqueue(context.Background(), &wire.SubscribeMessage{
			SubscribeID:        0,
			TrackAlias:         0,
			TrackNamespace:     []string{"namespace"},
			TrackName:          []byte("trackname"),
			SubscriberPriority: 0,
			GroupOrder:         0,
			FilterType:         0,
			StartGroup:         0,
			StartObject:        0,
			EndGroup:           0,
			Parameters: map[uint64]wire.Parameter{
				wire.AuthorizationParameterKey: &wire.StringParameter{
					Type:  wire.AuthorizationParameterKey,
					Value: "auth",
				},
			},
		}).DoAndReturn(func(_ context.Context, _ wire.ControlMessage) error {
			assert.NoError(t, s.handleUniStream(mp))
			assert.NoError(t, s.onSubscribeOk(&wire.SubscribeOkMessage{
				SubscribeID:     0,
				Expires:         0,
				GroupOrder:      1,
				ContentExists:   false,
				LargestGroupID:  0,
				LargestObjectID: 0,
				Parameters:      map[uint64]wire.Parameter{},
			}))
			return nil
		})
		rt, err := s.Subscribe(context.Background(), []string{"namespace"}, "trackname", "auth")
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})
}
