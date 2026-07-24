package moqtransport

import (
	"context"
	"io"
	"sync/atomic"
	"testing"

	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/qlog"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
	"golang.org/x/sync/errgroup"
)

func newSession(conn Connection, cs controlMessageStream, h Handler) *Session {
	return newSessionWithHandlers(conn, cs, h, nil)
}

func newSessionWithHandlers(conn Connection, cs controlMessageStream, h Handler, sh SubscribeHandler) *Session {
	s := &Session{
		Handler:                     h,
		SubscribeHandler:            sh,
		Qlogger:                     &qlog.Logger{},
		eg:                          &errgroup.Group{},
		ctx:                         nil,
		cancelCtx:                   nil,
		handshakeDoneCh:             make(chan struct{}),
		handshakeDone:               atomic.Bool{},
		logger:                      defaultLogger,
		conn:                        conn,
		controlStream:               cs,
		version:                     0,
		path:                        "/path",
		requestIDs:                  newRequestIDGenerator(uint64(conn.Perspective()), 100, 2),
		remoteTracks:                newRemoteTrackMap(),
		localTracks:                 newLocalTrackMap(),
		outgoingTrackStatusRequests: newTrackStatusRequestMap(),
		trackAliases:                newSequence(0, 1),
	}
	return s
}

func TestSession(t *testing.T) {
	t.Run("sends_client_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)

		cs.EXPECT().write(
			&wire.ClientSetupMessage{
				SupportedVersions: wire.SupportedVersions,
				SetupParameters: wire.KVPList{
					wire.KeyValuePair{
						Type:       wire.PathParameterKey,
						ValueBytes: []byte("/path"),
					},
				},
			},
		)

		s := newSession(conn, cs, nil)
		err := s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_client_setup_wt", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolWebTransport)

		cs.EXPECT().write(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters:   wire.KVPList{},
		},
		)

		s := newSession(conn, cs, nil)
		err := s.sendClientSetup()
		assert.NoError(t, err)
		assert.NotNil(t, s)
	})

	t.Run("sends_server_setup_quic", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveServer)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)

		s := newSession(conn, cs, nil)

		cs.EXPECT().write(&wire.ServerSetupMessage{
			SelectedVersion: wire.CurrentVersion,
			SetupParameters: wire.KVPList{},
		})

		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters: wire.KVPList{
				wire.KeyValuePair{
					Type:       wire.PathParameterKey,
					ValueBytes: []byte("/path"),
				},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("sends_server_setup_wt", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveServer)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolWebTransport)

		s := newSession(conn, cs, nil)

		cs.EXPECT().write(&wire.ServerSetupMessage{
			SelectedVersion: wire.CurrentVersion,
			SetupParameters: wire.KVPList{},
		})

		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters:   wire.KVPList{},
		})
		assert.NoError(t, err)
	})

	t.Run("rejects_quic_client_without_path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveServer)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)

		s := newSession(conn, cs, nil)
		err := s.receive(&wire.ClientSetupMessage{
			SupportedVersions: wire.SupportedVersions,
			SetupParameters:   wire.KVPList{},
		})
		assert.Error(t, err)
	})

	t.Run("sends_subscribe", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)

		s := newSession(conn, cs, nil)
		s.handshakeDone.Store(true)

		cs.EXPECT().write(&wire.SubscribeMessage{
			RequestID:          0,
			TrackNamespace:     []string{"namespace"},
			TrackName:          []byte("track"),
			SubscriberPriority: 128,
			GroupOrder:         1,
			Forward:            1,
			FilterType:         wire.FilterTypeLatestObject,
			StartLocation:      wire.Location{Group: 0, Object: 0},
			EndGroup:           0,
			Parameters: wire.KVPList{
				wire.KeyValuePair{
					Type:       wire.AuthorizationTokenParameterKey,
					ValueBytes: []byte("auth"),
				},
			},
		}).DoAndReturn(func(_ wire.ControlMessage) error {
			err := s.receive(&wire.SubscribeOkMessage{
				RequestID:       0,
				Expires:         0,
				GroupOrder:      0,
				ContentExists:   false,
				LargestLocation: wire.Location{},
				Parameters:      wire.KVPList{},
			})
			assert.NoError(t, err)
			return nil
		})
		rt, err := s.Subscribe(context.Background(), []string{"namespace"}, "track", WithAuthorizationToken("auth"))
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})

	t.Run("sends_subscribe_ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)

		// Create a SubscribeHandler that accepts the subscription
		sh := SubscribeHandlerFunc(func(w *SubscribeResponseWriter, m *SubscribeMessage) {
			assert.Equal(t, uint64(5), m.RequestID)
			assert.Equal(t, uint64(0), m.TrackAlias)
			assert.Equal(t, []string{"namespace"}, m.Namespace)
			assert.Equal(t, "track", m.Track)
			assert.Equal(t, "", m.Authorization)
			assert.NoError(t, w.Accept(WithLargestLocation(&Location{Group: 0, Object: 0})))
		})

		s := newSessionWithHandlers(conn, cs, nil, sh)
		s.handshakeDone.Store(true)
		cs.EXPECT().write(&wire.SubscribeOkMessage{
			RequestID:       5,
			Expires:         0,
			GroupOrder:      1,
			ContentExists:   true,
			LargestLocation: wire.Location{},
			Parameters:      wire.KVPList{},
		})
		err := s.receive(&wire.SubscribeMessage{
			RequestID:          5,
			TrackNamespace:     []string{"namespace"},
			TrackName:          []byte("track"),
			SubscriberPriority: 0,
			GroupOrder:         0,
			FilterType:         0,
			StartLocation: wire.Location{
				Group:  0,
				Object: 0,
			},
			EndGroup:   0,
			Parameters: wire.KVPList{},
		})
		assert.NoError(t, err)
	})

	t.Run("sends_subscribe_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)

		// Create a SubscribeHandler that rejects the subscription
		sh := SubscribeHandlerFunc(func(w *SubscribeResponseWriter, m *SubscribeMessage) {
			assert.Equal(t, uint64(0), m.RequestID)
			assert.Equal(t, uint64(0), m.TrackAlias)
			assert.Equal(t, []string{"namespace"}, m.Namespace)
			assert.Equal(t, "", m.Track)
			assert.Equal(t, "", m.Authorization)
			assert.NoError(t, w.Reject(ErrorCodeSubscribeTrackDoesNotExist, "track not found"))
		})

		s := newSessionWithHandlers(conn, cs, nil, sh)
		s.handshakeDone.Store(true)
		cs.EXPECT().write(&wire.SubscribeErrorMessage{
			RequestID:    0,
			ErrorCode:    uint64(ErrorCodeSubscribeTrackDoesNotExist),
			ReasonPhrase: "track not found",
		})
		err := s.receive(&wire.SubscribeMessage{
			RequestID:          0,
			TrackNamespace:     []string{"namespace"},
			TrackName:          []byte{},
			SubscriberPriority: 0,
			GroupOrder:         0,
			FilterType:         0,
			StartLocation: wire.Location{
				Group:  0,
				Object: 0,
			},
			EndGroup:   0,
			Parameters: wire.KVPList{},
		})
		assert.NoError(t, err)
	})

	t.Run("receives_objects_before_susbcribe_ok", func(t *testing.T) {
		t.Skip()
		ctrl := gomock.NewController(t)
		cs := NewMockControlMessageStream(ctrl)
		mh := NewMockHandler(ctrl)
		mp := NewMockObjectMessageParser(ctrl)
		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)

		mp.EXPECT().Type().Return(wire.StreamTypeSubgroupSIDExt).AnyTimes()
		mp.EXPECT().Identifier().Return(uint64(0)).AnyTimes()

		mp.EXPECT().Messages().Return(func(yield func(*wire.ObjectMessage, error) bool) {
			if !yield(&wire.ObjectMessage{
				TrackAlias:             2,
				GroupID:                0,
				SubgroupID:             0,
				ObjectID:               0,
				PublisherPriority:      0,
				ObjectExtensionHeaders: wire.KVPList{},
				ObjectStatus:           0,
				ObjectPayload:          []byte{},
			}, nil) {
				assert.Fail(t, "yield returned false")
				return
			}
			yield(nil, io.EOF)
		})

		s := newSession(conn, cs, mh)
		s.handshakeDone.Store(true)

		cs.EXPECT().write(&wire.SubscribeMessage{
			RequestID:          0,
			TrackNamespace:     []string{"namespace"},
			TrackName:          []byte("trackname"),
			SubscriberPriority: 128,
			GroupOrder:         1,
			Forward:            1,
			FilterType:         wire.FilterTypeLatestObject,
			StartLocation:      wire.Location{Group: 0, Object: 0},
			EndGroup:           0,
			Parameters: wire.KVPList{
				wire.KeyValuePair{
					Type:       wire.AuthorizationTokenParameterKey,
					ValueBytes: []byte("auth"),
				},
			},
		}).DoAndReturn(func(_ wire.ControlMessage) error {
			assert.NoError(t, s.handleUniStream(mp))
			assert.NoError(t, s.onSubscribeOk(&wire.SubscribeOkMessage{
				RequestID:       0,
				TrackAlias:      2,
				Expires:         0,
				GroupOrder:      1,
				ContentExists:   false,
				LargestLocation: wire.Location{},
				Parameters:      wire.KVPList{},
			}))
			return nil
		})
		rt, err := s.Subscribe(context.Background(), []string{"namespace"}, "trackname", WithAuthorizationToken("auth"))
		assert.NoError(t, err)
		assert.NotNil(t, rt)
	})
}

func TestSession_UpdateSubscription(t *testing.T) {
	t.Run("UpdateSubscription sends SUBSCRIBE_UPDATE message", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)
		cs := NewMockControlMessageStream(ctrl)
		h := HandlerFunc(func(rw ResponseWriter, m *Message) {})

		s := newSession(conn, cs, h)
		_ = s.remoteTracks.addPending(123, &RemoteTrack{requestID: 123})
		_, err := s.remoteTracks.confirm(123)
		assert.NoError(t, err)

		// Expect SUBSCRIBE_UPDATE message to be written
		cs.EXPECT().write(&wire.SubscribeUpdateMessage{
			RequestID:          123,
			StartLocation:      wire.Location{Group: 100, Object: 5},
			EndGroup:           200,
			SubscriberPriority: 64,
			Forward:            1,
			Parameters:         wire.KVPList{},
		}).Return(nil)

		// Test UpdateSubscription
		err = s.UpdateSubscription(context.Background(), 123,
			WithUpdateStartLocation(Location{Group: 100, Object: 5}),
			WithUpdateEndGroup(200),
			WithUpdateSubscriberPriority(64),
			WithUpdateForward(true),
			WithUpdateParameters(KVPList{}),
		)
		assert.NoError(t, err)
	})

	t.Run("UpdateSubscription returns error for unknown request ID", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		conn := NewMockConnection(ctrl)
		conn.EXPECT().Perspective().AnyTimes().Return(PerspectiveClient)
		conn.EXPECT().Protocol().AnyTimes().Return(ProtocolQUIC)
		cs := NewMockControlMessageStream(ctrl)
		h := HandlerFunc(func(rw ResponseWriter, m *Message) {})

		s := newSession(conn, cs, h)

		err := s.UpdateSubscription(context.Background(), 999)
		assert.Error(t, err)
		assert.Equal(t, errUnknownRequestID, err)
	})
}

func TestRemoteTrack_UpdateSubscription(t *testing.T) {
	t.Run("RemoteTrack UpdateSubscription calls session method", func(t *testing.T) {
		callCount := 0
		updateFunc := func(ctx context.Context, options ...SubscribeUpdateOption) error {
			callCount++
			// Convert options back to struct for testing
			opts := &SubscribeUpdateOptions{
				StartLocation:      Location{Group: 0, Object: 0},
				EndGroup:           0,
				SubscriberPriority: 128,
				Forward:            true,
				Parameters:         KVPList{},
			}
			for _, option := range options {
				option(opts)
			}
			assert.Equal(t, uint64(150), opts.StartLocation.Group)
			return nil
		}

		rt := newRemoteTrack(123, nil, updateFunc)

		err := rt.UpdateSubscription(context.Background(), WithUpdateStartLocation(Location{Group: 150, Object: 0}))
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("RemoteTrack UpdateSubscription returns error when updateFunc is nil", func(t *testing.T) {
		rt := newRemoteTrack(123, nil, nil)

		err := rt.UpdateSubscription(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update function not available")
	})
}
