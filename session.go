package moqtransport

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"slices"
	"sync/atomic"

	"github.com/mengelbart/moqtransport/internal/wire"
)

var (
	errUnknownAnnouncementNamespace     = errors.New("unknown announcement namespace")
	errMaxRequestIDViolated             = errors.New("max request ID violated")
	errClientReceivedClientSetup        = errors.New("client received client setup message")
	errServerReceveidServerSetup        = errors.New("server received server setup message")
	errIncompatibleVersions             = errors.New("incompatible versions")
	errUnexpectedMessageType            = errors.New("unexpected message type")
	errUnexpectedMessageTypeBeforeSetup = errors.New("unexpected message type before setup")
	errUnknownSubscribeAnnouncesPrefix  = errors.New("unknown subscribe_announces prefix")
	errUnknownTrackAlias                = errors.New("unknown track alias")
	errMissingPathParameter             = errors.New("missing path parameter")
	errUnexpectedPathParameter          = errors.New("unexpected path parameter on QUIC connection")
	errUnknownTrackStatusRequest        = errors.New("got unexpected track status requrest")
)

type controlMessageQueue[T any] interface {
	enqueue(context.Context, T) error
	dequeue(context.Context) (T, error)
}

type objectMessageParser interface {
	Type() wire.StreamType
	Identifier() uint64
	Messages() iter.Seq2[*wire.ObjectMessage, error]
}

// A Session is an endpoint of a MoQ Session session.
type Session struct {
	Protocol    Protocol
	Perspective Perspective

	logger *slog.Logger

	handshakeDoneCh chan struct{}

	ctrlMsgSendQueue    controlMessageQueue[wire.ControlMessage]
	ctrlMsgReceiveQueue controlMessageQueue[*Message]

	version wire.Version
	path    string

	localMaxRequestID atomic.Uint64

	requestIDs             *requestIDGenerator
	highestRequestsBlocked atomic.Uint64

	outgoingAnnouncements *announcementMap
	incomingAnnouncements *announcementMap

	pendingOutgointAnnouncementSubscriptions *announcementSubscriptionMap
	pendingIncomingAnnouncementSubscriptions *announcementSubscriptionMap

	trackAliases *sequence
	remoteTracks *remoteTrackMap
	localTracks  *localTrackMap

	outgoingTrackStatusRequests *trackStatusRequestMap
}

func NewSession(proto Protocol, perspective Perspective, initMaxRequestID uint64) *Session {
	s := &Session{
		Protocol:                                 proto,
		Perspective:                              perspective,
		logger:                                   defaultLogger.With("perspective", perspective),
		handshakeDoneCh:                          make(chan struct{}),
		ctrlMsgSendQueue:                         newQueue[wire.ControlMessage](),
		ctrlMsgReceiveQueue:                      newQueue[*Message](),
		version:                                  0,
		path:                                     "",
		requestIDs:                               newRequestIDGenerator(uint64(perspective), 2, 0),
		outgoingAnnouncements:                    newAnnouncementMap(),
		incomingAnnouncements:                    newAnnouncementMap(),
		pendingOutgointAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		pendingIncomingAnnouncementSubscriptions: newAnnouncementSubscriptionMap(),
		highestRequestsBlocked:                   atomic.Uint64{},
		trackAliases:                             newSequence(0, 1),
		remoteTracks:                             newRemoteTrackMap(),
		localTracks:                              newLocalTrackMap(),
		outgoingTrackStatusRequests:              newTrackStatusRequestMap(),
		localMaxRequestID:                        atomic.Uint64{},
	}
	s.localMaxRequestID.Store(initMaxRequestID)
	return s
}

func (s *Session) sendControlMessage(ctx context.Context) (wire.ControlMessage, error) {
	return s.ctrlMsgSendQueue.dequeue(ctx)
}

func (s *Session) readControlMessage(ctx context.Context) (*Message, error) {
	return s.ctrlMsgReceiveQueue.dequeue(ctx)
}

func (s *Session) handleUniStream(parser objectMessageParser) error {
	if parser.Type() == wire.StreamTypeFetch {
		return s.readFetchStream(parser)
	}
	return s.readSubgroupStream(parser)
}

func (s *Session) readFetchStream(parser objectMessageParser) error {
	s.logger.Info("reading fetch stream")
	rt, ok := s.remoteTrackByRequestID(parser.Identifier())
	if !ok {
		return errUnknownRequestID
	}
	return rt.readFetchStream(parser)
}

func (s *Session) readSubgroupStream(parser objectMessageParser) error {
	s.logger.Info("reading subgroup")
	rt, ok := s.remoteTrackByTrackAlias(parser.Identifier())
	if !ok {
		return errUnknownRequestID
	}
	return rt.readSubgroupStream(parser)
}

func (s *Session) receiveDatagram(msg *wire.ObjectDatagramMessage) error {
	subscription, ok := s.remoteTrackByTrackAlias(msg.TrackAlias)
	if !ok {
		return errUnknownTrackAlias
	}
	subscription.push(&Object{
		GroupID:              msg.GroupID,
		ObjectID:             msg.ObjectID,
		ForwardingPreference: ObjectForwardingPreferenceDatagarm,
		Payload:              msg.ObjectPayload,
	})
	return nil
}

func (s *Session) addLocalTrack(lt *localTrack) error {
	if lt.requestID >= s.localMaxRequestID.Load() {
		return errMaxRequestIDViolated
	}
	// Update max request ID for peer
	oldMax := s.localMaxRequestID.Load()
	if lt.requestID >= oldMax/2 {
		newMax := 2 * oldMax
		s.localMaxRequestID.CompareAndSwap(oldMax, newMax)
		if err := s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.MaxRequestIDMessage{
			RequestID: newMax,
		}); err != nil {
			s.logger.Warn("skipping sending of max_request_id due to control message queue overflow")
		}
	}
	ok := s.localTracks.addPending(lt)
	if !ok {
		return errDuplicateRequestID
	}
	return nil
}

func (s *Session) remoteTrackByRequestID(id uint64) (*RemoteTrack, bool) {
	sub, ok := s.remoteTracks.findByRequestID(id)
	return sub, ok
}

func (s *Session) remoteTrackByTrackAlias(alias uint64) (*RemoteTrack, bool) {
	sub, ok := s.remoteTracks.findByTrackAlias(alias)
	return sub, ok
}

func (s *Session) handshakeDone() bool {
	select {
	case <-s.handshakeDoneCh:
		return true
	default:
		return false
	}
}

func (s *Session) waitForHandshakeDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.handshakeDoneCh:
		return nil
	}
}

func (s *Session) getRequestID() (uint64, error) {
	requestID, err := s.requestIDs.next()
	if err != nil {
		if err == errRequestIDblocked {
			if queueErr := s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.RequestsBlockedMessage{
				MaximumRequestID: requestID,
			}); queueErr != nil {
				s.logger.Warn("skipping sending of requests_blocked message", "error", queueErr)
			}
		}
	}
	return requestID, err
}

// Path returns the path of the MoQ session which was exchanged during the
// handshake when using QUIC.
func (s *Session) Path() string {
	return s.path
}

// Session message senders

func (s *Session) sendClientSetup() error {
	params := wire.KVPList{
		wire.KeyValuePair{
			Type:        wire.MaxRequestIDParameterKey,
			ValueVarInt: s.localMaxRequestID.Load(),
		},
	}
	if s.Protocol == ProtocolQUIC {
		path := s.path
		params = append(params, wire.KeyValuePair{
			Type:       wire.PathParameterKey,
			ValueBytes: []byte(path),
		})
	}
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.ClientSetupMessage{
		SupportedVersions: wire.SupportedVersions,
		SetupParameters:   params,
	})
}

// Subscribe subscribes to track in namespace. It blocks until a response from
// the peer was received or ctx is cancelled.
func (s *Session) Subscribe(
	ctx context.Context,
	namespace []string,
	name string,
	auth string,
) (*RemoteTrack, error) {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	requestID, err := s.getRequestID()
	if err != nil {
		return nil, err
	}
	rt := newRemoteTrack(requestID, func() error {
		return s.unsubscribe(requestID)
	})
	trackAlias := s.trackAliases.next()
	if err = s.remoteTracks.addPendingWithAlias(requestID, trackAlias, rt); err != nil {
		return nil, err
	}
	cm := &wire.SubscribeMessage{
		RequestID:          requestID,
		TrackAlias:         trackAlias,
		TrackNamespace:     namespace,
		TrackName:          []byte(name),
		SubscriberPriority: 0,
		GroupOrder:         0,
		FilterType:         0,
		StartLocation: wire.Location{
			Group:  0,
			Object: 0,
		},
		EndGroup:   0,
		Parameters: wire.KVPList{},
	}
	if len(auth) > 0 {
		cm.Parameters = append(cm.Parameters, wire.KeyValuePair{
			Type:       wire.AuthorizationTokenParameterKey,
			ValueBytes: []byte(auth),
		})
	}
	if err := s.ctrlMsgSendQueue.enqueue(ctx, cm); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		err = context.Cause(ctx)
	case err = <-rt.responseChan:
	}
	if err != nil {
		s.remoteTracks.reject(requestID)
		if closeErr := rt.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return rt, nil
}

func (s *Session) acceptSubscription(id uint64) error {
	_, ok := s.localTracks.confirm(id)
	if !ok {
		return errUnknownRequestID
	}
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.SubscribeOkMessage{
		RequestID:     id,
		Expires:       0,
		GroupOrder:    1,
		ContentExists: false,
		LargestLocation: wire.Location{
			Group:  0,
			Object: 0,
		},
		Parameters: wire.KVPList{},
	})
}

func (s *Session) rejectSubscription(id uint64, errorCode uint64, reason string) error {
	lt, ok := s.localTracks.reject(id)
	if !ok {
		return errUnknownRequestID
	}
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.SubscribeErrorMessage{
		RequestID:    lt.requestID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
		TrackAlias:   lt.trackAlias,
	})
}

func (s *Session) unsubscribe(id uint64) error {
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.UnsubscribeMessage{
		RequestID: id,
	})
}

// SubscribeUpdate sends a SUBSCRIBE_UPDATE message to modify an existing subscription.
// The requestID must correspond to an active subscription.
func (s *Session) SubscribeUpdate(
	ctx context.Context,
	requestID uint64,
	priority uint8,
	startGroup uint64,
	startObject uint64,
	endGroup uint64,
	forward uint8,
) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	
	// Verify the subscription exists
	rt, ok := s.remoteTracks.findByRequestID(requestID)
	if !ok {
		return errUnknownRequestID
	}
	
	// Create and send the SUBSCRIBE_UPDATE message
	sum := &wire.SubscribeUpdateMessage{
		RequestID: requestID,
		StartLocation: wire.Location{
			Group:  startGroup,
			Object: startObject,
		},
		EndGroup:           endGroup,
		SubscriberPriority: priority,
		Forward:            forward,
		Parameters:         wire.KVPList{},
	}
	
	if err := s.ctrlMsgSendQueue.enqueue(ctx, sum); err != nil {
		return err
	}
	
	// Log the update
	s.logger.Info("sent SUBSCRIBE_UPDATE", 
		"requestID", requestID,
		"priority", priority,
		"startGroup", startGroup,
		"startObject", startObject,
		"endGroup", endGroup,
		"forward", forward,
		"track", rt,
	)
	
	return nil
}

func (s *Session) subscriptionDone(id, code, count uint64, reason string) error {
	lt, ok := s.localTracks.delete(id)
	if !ok {
		return errUnknownRequestID
	}
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.SubscribeDoneMessage{
		RequestID:    lt.requestID,
		StatusCode:   code,
		StreamCount:  count,
		ReasonPhrase: reason,
	})
}

// Fetch fetches track in namespace from the peer. It blocks until a response
// from the peer was received or ctx is cancelled.
func (s *Session) Fetch(
	ctx context.Context,
	namespace []string,
	track string,
) (*RemoteTrack, error) {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	requestID, err := s.getRequestID()
	if err != nil {
		return nil, err
	}
	rt := newRemoteTrack(requestID, func() error {
		return s.fetchCancel(requestID)
	})
	if err := s.remoteTracks.addPending(requestID, rt); err != nil {
		var tooManySubscribes errRequestsBlocked
		if errors.As(err, &tooManySubscribes) {
			previous := s.highestRequestsBlocked.Swap(tooManySubscribes.maxRequestID)
			if previous < tooManySubscribes.maxRequestID {
				if queueErr := s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.RequestsBlockedMessage{
					MaximumRequestID: tooManySubscribes.maxRequestID,
				}); queueErr != nil {
					s.logger.Warn("skipping sending of requests_blocked message", "error", queueErr)
				}
			}
		}
		return nil, err
	}
	cm := &wire.FetchMessage{
		RequestID:          requestID,
		SubscriberPriority: 0,
		GroupOrder:         0,
		FetchType:          wire.FetchTypeStandalone,
		TrackNamespace:     namespace,
		TrackName:          []byte(track),
		StartGroup:         0,
		StartObject:        0,
		EndGroup:           0,
		EndObject:          0,
		JoiningSubscribeID: 0,
		JoiningStart:       0,
		Parameters:         wire.KVPList{},
	}
	if err := s.ctrlMsgSendQueue.enqueue(ctx, cm); err != nil {
		_, _ = s.remoteTracks.reject(requestID)
		return nil, err
	}
	select {
	case <-ctx.Done():
		err = context.Cause(ctx)
	case err = <-rt.responseChan:
	}
	if err != nil {
		s.remoteTracks.reject(requestID)
		if closeErr := rt.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return rt, nil
}

func (s *Session) acceptFetch(requestID uint64) error {
	_, ok := s.localTracks.confirm(requestID)
	if !ok {
		return errUnknownRequestID
	}
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.FetchOkMessage{
		RequestID:  requestID,
		GroupOrder: 1,
		EndOfTrack: 0,
		EndLocation: wire.Location{
			Group:  0,
			Object: 0,
		},
		SubscribeParameters: wire.KVPList{},
	})
}

func (s *Session) rejectFetch(id uint64, errorCode uint64, reason string) error {
	lt, ok := s.localTracks.reject(id)
	if !ok {
		return errUnknownRequestID
	}
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.FetchErrorMessage{
		RequestID:    lt.requestID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
	})

}

func (s *Session) fetchCancel(id uint64) error {
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.FetchCancelMessage{
		RequestID: id,
	})
}

func (s *Session) RequestTrackStatus(ctx context.Context, namespace []string, track string) (*TrackStatus, error) {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return nil, err
	}
	requestID, err := s.getRequestID()
	if err != nil {
		return nil, err
	}
	tsr := &trackStatusRequest{
		requestID: requestID,
		namespace: namespace,
		trackname: track,
		response:  make(chan *TrackStatus, 1),
	}

	s.outgoingTrackStatusRequests.add(tsr)
	tsrm := &wire.TrackStatusRequestMessage{
		TrackNamespace: namespace,
		TrackName:      []byte(track),
	}
	if err := s.ctrlMsgSendQueue.enqueue(ctx, tsrm); err != nil {
		_, _ = s.outgoingTrackStatusRequests.delete(tsrm.RequestID)
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	case status := <-tsr.response:
		return status, nil
	}
}

func (s *Session) sendTrackStatus(ts TrackStatus) error {
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.TrackStatusMessage{
		StatusCode:      ts.StatusCode,
		RequestID:       0,
		LargestLocation: wire.Location{},
		Parameters:      wire.KVPList{},
	})
}

// Announce announces namespace to the peer. It blocks until a response from the
// peer was received or ctx is cancelled and returns an error if the
// announcement was rejected.
func (s *Session) Announce(ctx context.Context, namespace []string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	requestID, err := s.getRequestID()
	if err != nil {
		return err
	}
	a := &announcement{
		requestID:  requestID,
		namespace:  namespace,
		parameters: wire.KVPList{},
		response:   make(chan error, 1),
	}
	s.outgoingAnnouncements.add(a)
	am := &wire.AnnounceMessage{
		RequestID:      a.requestID,
		TrackNamespace: a.namespace,
		Parameters:     a.parameters,
	}
	if err := s.ctrlMsgSendQueue.enqueue(ctx, am); err != nil {
		_, _ = s.outgoingAnnouncements.reject(a.requestID)
		return err
	}
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case res := <-a.response:
		return res
	}
}

func (s *Session) acceptAnnouncement(requestID uint64) error {
	if _, err := s.incomingAnnouncements.confirmAndGet(requestID); err != nil {
		return err
	}
	if err := s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.AnnounceOkMessage{
		RequestID: requestID,
	}); err != nil {
		return err
	}
	return nil
}

func (s *Session) rejectAnnouncement(requestID uint64, c uint64, r string) error {
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.AnnounceErrorMessage{
		RequestID:    requestID,
		ErrorCode:    c,
		ReasonPhrase: r,
	})
}

func (s *Session) Unannounce(ctx context.Context, namespace []string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	if ok := s.outgoingAnnouncements.delete(namespace); ok {
		return errUnknownAnnouncementNamespace
	}
	u := &wire.UnannounceMessage{
		TrackNamespace: namespace,
	}
	return s.ctrlMsgSendQueue.enqueue(ctx, u)
}

func (s *Session) AnnounceCancel(ctx context.Context, namespace []string, errorCode uint64, reason string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	if !s.incomingAnnouncements.delete(namespace) {
		return errUnknownAnnouncementNamespace
	}
	acm := &wire.AnnounceCancelMessage{
		TrackNamespace: namespace,
		ErrorCode:      errorCode,
		ReasonPhrase:   reason,
	}
	return s.ctrlMsgSendQueue.enqueue(ctx, acm)
}

// SubscribeAnnouncements subscribes to announcements of namespaces with prefix.
// It blocks until a response from the peer is received or ctx is cancelled.
func (s *Session) SubscribeAnnouncements(ctx context.Context, prefix []string) error {
	if err := s.waitForHandshakeDone(ctx); err != nil {
		return err
	}
	requestID, err := s.getRequestID()
	if err != nil {
		return err
	}
	as := &announcementSubscription{
		requestID: requestID,
		namespace: prefix,
		response:  make(chan announcementSubscriptionResponse, 1),
	}
	s.pendingOutgointAnnouncementSubscriptions.add(as)
	sam := &wire.SubscribeAnnouncesMessage{
		RequestID:            as.requestID,
		TrackNamespacePrefix: as.namespace,
		Parameters:           wire.KVPList{},
	}
	if err := s.ctrlMsgSendQueue.enqueue(ctx, sam); err != nil {
		_, _ = s.pendingOutgointAnnouncementSubscriptions.deleteByID(as.requestID)
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-as.response:
		return resp.err
	}
}

func (s *Session) acceptAnnouncementSubscription(requestID uint64) error {
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.SubscribeAnnouncesOkMessage{
		RequestID: requestID,
	})
}

func (s *Session) rejectAnnouncementSubscription(requestID uint64, c uint64, r string) error {
	return s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.SubscribeAnnouncesErrorMessage{
		RequestID:    requestID,
		ErrorCode:    c,
		ReasonPhrase: r,
	})
}

func (s *Session) UnsubscribeAnnouncements(ctx context.Context, namespace []string) error {
	s.pendingOutgointAnnouncementSubscriptions.delete(namespace)
	uam := &wire.UnsubscribeAnnouncesMessage{
		TrackNamespacePrefix: namespace,
	}
	return s.ctrlMsgSendQueue.enqueue(ctx, uam)
}

// Session message handlers

func (s *Session) receive(msg wire.ControlMessage) error {
	s.logger.Info("received message", "type", msg.Type().String(), "msg", msg)

	if !s.handshakeDone() {
		switch m := msg.(type) {
		case *wire.ClientSetupMessage:
			return s.onClientSetup(m)
		case *wire.ServerSetupMessage:
			return s.onServerSetup(m)
		}
		return errUnexpectedMessageTypeBeforeSetup
	}

	var err error
	switch m := msg.(type) {
	case *wire.GoAwayMessage:
		err = s.onGoAway(m)
	case *wire.MaxRequestIDMessage:
		err = s.onMaxRequestID(m)
	case *wire.RequestsBlockedMessage:
		err = s.onRequestsBlocked(m)
	case *wire.SubscribeMessage:
		err = s.onSubscribe(m)
	case *wire.SubscribeOkMessage:
		err = s.onSubscribeOk(m)
	case *wire.SubscribeErrorMessage:
		err = s.onSubscribeError(m)
	case *wire.SubscribeUpdateMessage:
		err = s.onSubscribeUpdate(m)
	case *wire.UnsubscribeMessage:
		err = s.onUnsubscribe(m)
	case *wire.SubscribeDoneMessage:
		err = s.onSubscribeDone(m)
	case *wire.FetchMessage:
		err = s.onFetch(m)
	case *wire.FetchOkMessage:
		err = s.onFetchOk(m)
	case *wire.FetchErrorMessage:
		err = s.onFetchError(m)
	case *wire.FetchCancelMessage:
		err = s.onFetchCancel(m)
	case *wire.TrackStatusRequestMessage:
		err = s.onTrackStatusRequest(m)
	case *wire.TrackStatusMessage:
		err = s.onTrackStatus(m)
	case *wire.AnnounceMessage:
		err = s.onAnnounce(m)
	case *wire.AnnounceOkMessage:
		err = s.onAnnounceOk(m)
	case *wire.AnnounceErrorMessage:
		err = s.onAnnounceError(m)
	case *wire.UnannounceMessage:
		err = s.onUnannounce(m)
	case *wire.AnnounceCancelMessage:
		err = s.onAnnounceCancel(m)
	case *wire.SubscribeAnnouncesMessage:
		err = s.onSubscribeAnnounces(m)
	case *wire.SubscribeAnnouncesOkMessage:
		err = s.onSubscribeAnnouncesOk(m)
	case *wire.SubscribeAnnouncesErrorMessage:
		err = s.onSubscribeAnnouncesError(m)
	case *wire.UnsubscribeAnnouncesMessage:
		err = s.onUnsubscribeAnnounces(m)
	default:
		err = errUnexpectedMessageType
	}
	return err
}

func (s *Session) onClientSetup(m *wire.ClientSetupMessage) error {
	if s.Perspective != PerspectiveServer {
		return errClientReceivedClientSetup
	}
	selectedVersion := -1
	for _, v := range slices.Backward(wire.SupportedVersions) {
		if slices.Contains(m.SupportedVersions, v) {
			selectedVersion = int(v)
			break
		}
	}
	if selectedVersion == -1 {
		return errIncompatibleVersions
	}
	s.version = wire.Version(selectedVersion)

	path, err := validatePathParameter(m.SetupParameters, s.Protocol == ProtocolQUIC)
	if err != nil {
		return err
	}
	s.path = path

	remoteMaxRequestID := getMaxRequestIDParameter(m.SetupParameters)
	if remoteMaxRequestID > 0 {
		if err := s.requestIDs.setMax(remoteMaxRequestID); err != nil {
			return err
		}
	}

	if err := s.ctrlMsgSendQueue.enqueue(context.Background(), &wire.ServerSetupMessage{
		SelectedVersion: wire.Version(selectedVersion),
		SetupParameters: wire.KVPList{
			wire.KeyValuePair{
				Type:        wire.MaxRequestIDParameterKey,
				ValueVarInt: 100,
			},
		},
	}); err != nil {
		return err
	}
	close(s.handshakeDoneCh)
	return nil
}

func (s *Session) onServerSetup(m *wire.ServerSetupMessage) (err error) {
	if s.Perspective != PerspectiveClient {
		return errServerReceveidServerSetup
	}

	if !slices.Contains(wire.SupportedVersions, m.SelectedVersion) {
		return errIncompatibleVersions
	}
	s.version = m.SelectedVersion

	remoteMaxRequestID := getMaxRequestIDParameter(m.SetupParameters)
	if err := s.requestIDs.setMax(remoteMaxRequestID); err != nil {
		return err
	}
	close(s.handshakeDoneCh)
	return nil
}

func (s *Session) onGoAway(msg *wire.GoAwayMessage) error {
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), &Message{
		Method:        MessageGoAway,
		NewSessionURI: msg.NewSessionURI,
	})
}

func (s *Session) onMaxRequestID(msg *wire.MaxRequestIDMessage) error {
	return s.requestIDs.setMax(msg.RequestID)
}

func (s *Session) onRequestsBlocked(msg *wire.RequestsBlockedMessage) error {
	s.logger.Info("received subscribes blocked message", "max_request_id", msg.MaximumRequestID)
	return nil
}

func (s *Session) onSubscribe(msg *wire.SubscribeMessage) error {
	auth, err := validateAuthParameter(msg.Parameters)
	if err != nil {
		return err
	}
	m := &Message{
		Method:        MessageSubscribe,
		RequestID:     msg.RequestID,
		TrackAlias:    msg.TrackAlias,
		Namespace:     msg.TrackNamespace,
		Track:         string(msg.TrackName),
		Authorization: auth,
		NewSessionURI: "",
		ErrorCode:     0,
		ReasonPhrase:  "",
	}
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), m)
}

func (s *Session) onSubscribeOk(msg *wire.SubscribeOkMessage) error {
	rt, ok := s.remoteTracks.confirm(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	select {
	case rt.responseChan <- nil:
	default:
		s.logger.Warn("dropping unhandled SubscribeOk response")
		if err := rt.Close(); err != nil {
			s.logger.Error("failed to unsubscribe from unhandled subscription", "error", err)
		}
	}
	return nil
}

func (s *Session) onSubscribeError(msg *wire.SubscribeErrorMessage) error {
	sub, ok := s.remoteTracks.reject(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	select {
	case sub.responseChan <- ProtocolError{
		code:    msg.ErrorCode,
		message: msg.ReasonPhrase,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeError response")
	}
	return nil
}

func (s *Session) onSubscribeUpdate(msg *wire.SubscribeUpdateMessage) error {
	// Find the local track for this request ID
	lt, ok := s.localTracks.findByID(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	
	// Create a message for the application handler
	m := &Message{
		Method:             MessageSubscribeUpdate,
		RequestID:          msg.RequestID,
		TrackAlias:         lt.trackAlias,
		SubscriberPriority: msg.SubscriberPriority,
		StartGroup:         msg.StartLocation.Group,
		StartObject:        msg.StartLocation.Object,
		EndGroup:           msg.EndGroup,
		Forward:            msg.Forward,
	}
	
	// Pass to the application handler
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), m)
}

// TODO: Maybe don't immediately close the track and give app a chance to react
// first?
func (s *Session) onUnsubscribe(msg *wire.UnsubscribeMessage) error {
	lt, ok := s.localTracks.findByID(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	lt.unsubscribe()
	return nil
}

func (s *Session) onSubscribeDone(msg *wire.SubscribeDoneMessage) error {
	sub, ok := s.remoteTracks.findByRequestID(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	sub.done(msg.StatusCode, msg.ReasonPhrase)
	// TODO: Remove subscription from outgoingSubscriptions map, but maybe only
	// after timeout to wait for late coming objects?
	return nil
}

func (s *Session) onFetch(msg *wire.FetchMessage) error {
	m := &Message{
		Method:        MessageFetch,
		Namespace:     msg.TrackNamespace,
		Track:         string(msg.TrackName),
		RequestID:     msg.RequestID,
		TrackAlias:    0,
		Authorization: "",
		NewSessionURI: "",
		ErrorCode:     0,
		ReasonPhrase:  "",
	}
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), m)
}

func (s *Session) onFetchOk(msg *wire.FetchOkMessage) error {
	rt, ok := s.remoteTracks.confirm(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	select {
	case rt.responseChan <- nil:
	default:
		s.logger.Info("dropping unhandled fetchOk response")
		if err := rt.Close(); err != nil {
			s.logger.Error("failed to unsubscribe from unhandled fetch", "error", err)
		}
	}
	return nil
}

func (s *Session) onFetchError(msg *wire.FetchErrorMessage) error {
	rt, ok := s.remoteTracks.reject(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	select {
	case rt.responseChan <- ProtocolError{
		code:    msg.ErrorCode,
		message: msg.ReasonPhrase,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeError response")
	}
	return nil
}

func (s *Session) onFetchCancel(msg *wire.FetchCancelMessage) error {
	lt, ok := s.localTracks.delete(msg.RequestID)
	if !ok {
		return errUnknownRequestID
	}
	lt.unsubscribe()
	return nil
}

func (s *Session) onTrackStatusRequest(msg *wire.TrackStatusRequestMessage) error {
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), &Message{
		Method:    MessageTrackStatusRequest,
		Namespace: msg.TrackNamespace,
		Track:     string(msg.TrackName),
	})
}

func (s *Session) onTrackStatus(msg *wire.TrackStatusMessage) error {
	tsr, ok := s.outgoingTrackStatusRequests.delete(msg.RequestID)
	if !ok {
		return errUnknownTrackStatusRequest
	}
	select {
	case tsr.response <- &TrackStatus{
		Namespace:    tsr.namespace,
		Trackname:    tsr.trackname,
		StatusCode:   msg.StatusCode,
		LastGroupID:  msg.LargestLocation.Group,
		LastObjectID: msg.LargestLocation.Object,
	}:
	default:
		s.logger.Info("dropping unhandled track status")
	}
	return nil
}

func (s *Session) onAnnounce(msg *wire.AnnounceMessage) error {
	a := &announcement{
		requestID:  msg.RequestID,
		namespace:  msg.TrackNamespace,
		parameters: msg.Parameters,
		response:   make(chan error),
	}
	s.incomingAnnouncements.add(a)
	message := &Message{
		RequestID: msg.RequestID,
		Method:    MessageAnnounce,
		Namespace: a.namespace,
	}
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), message)
}

func (s *Session) onAnnounceOk(msg *wire.AnnounceOkMessage) error {
	announcement, err := s.outgoingAnnouncements.confirmAndGet(msg.RequestID)
	if err != nil {
		return errUnknownAnnouncement
	}
	select {
	case announcement.response <- nil:
	default:
		s.logger.Info("dopping unhandled AnnounceOk response")
	}
	return nil
}

func (s *Session) onAnnounceError(msg *wire.AnnounceErrorMessage) error {
	announcement, ok := s.outgoingAnnouncements.reject(msg.RequestID)
	if !ok {
		return errUnknownAnnouncement
	}
	select {
	case announcement.response <- ProtocolError{
		code:    msg.ErrorCode,
		message: msg.ReasonPhrase,
	}:
	default:
		s.logger.Info("dropping unhandled AnnounceError response")
	}
	return nil
}

func (s *Session) onUnannounce(msg *wire.UnannounceMessage) error {
	if !s.incomingAnnouncements.delete(msg.TrackNamespace) {
		return errUnknownAnnouncement
	}
	req := &Message{
		Method:    MessageUnannounce,
		Namespace: msg.TrackNamespace,
	}
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), req)
}

func (s *Session) onAnnounceCancel(msg *wire.AnnounceCancelMessage) error {
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), &Message{
		Method:       MessageAnnounceCancel,
		Namespace:    msg.TrackNamespace,
		ErrorCode:    msg.ErrorCode,
		ReasonPhrase: msg.ReasonPhrase,
	})
}

func (s *Session) onSubscribeAnnounces(msg *wire.SubscribeAnnouncesMessage) error {
	s.pendingIncomingAnnouncementSubscriptions.add(&announcementSubscription{
		requestID: msg.RequestID,
		namespace: msg.TrackNamespacePrefix,
	})
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), &Message{
		RequestID: msg.RequestID,
		Method:    MessageSubscribeAnnounces,
		Namespace: msg.TrackNamespacePrefix,
	})
}

func (s *Session) onSubscribeAnnouncesOk(msg *wire.SubscribeAnnouncesOkMessage) error {
	as, ok := s.pendingOutgointAnnouncementSubscriptions.deleteByID(msg.RequestID)
	if !ok {
		return errUnknownSubscribeAnnouncesPrefix
	}
	select {
	case as.response <- announcementSubscriptionResponse{
		err: nil,
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeAnnounces response")
	}
	return nil
}

func (s *Session) onSubscribeAnnouncesError(msg *wire.SubscribeAnnouncesErrorMessage) error {
	as, ok := s.pendingOutgointAnnouncementSubscriptions.deleteByID(msg.RequestID)
	if !ok {
		return errUnknownSubscribeAnnouncesPrefix
	}
	select {
	case as.response <- announcementSubscriptionResponse{
		err: ProtocolError{
			code:    msg.ErrorCode,
			message: msg.ReasonPhrase,
		},
	}:
	default:
		s.logger.Info("dropping unhandled SubscribeAnnounces response")
	}
	return nil
}

func (s *Session) onUnsubscribeAnnounces(msg *wire.UnsubscribeAnnouncesMessage) error {
	return s.ctrlMsgReceiveQueue.enqueue(context.Background(), &Message{
		Method:    MessageUnsubscribeAnnounces,
		Namespace: msg.TrackNamespacePrefix,
	})
}
