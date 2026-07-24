package moqtransport

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"sync/atomic"

	"github.com/mengelbart/moqtransport/internal/slices"
	"github.com/mengelbart/moqtransport/internal/wire"
	"github.com/mengelbart/qlog"
	"github.com/mengelbart/qlog/moqt"
	"golang.org/x/sync/errgroup"
)

var (
	errMaxRequestIDViolated             = errors.New("max request ID violated")
	errClientReceivedClientSetup        = errors.New("client received client setup message")
	errServerReceveidServerSetup        = errors.New("server received server setup message")
	errIncompatibleVersions             = errors.New("incompatible versions")
	errUnexpectedMessageType            = errors.New("unexpected message type")
	errUnexpectedMessageTypeBeforeSetup = errors.New("unexpected message type before setup")
	errUnknownTrackAlias                = errors.New("unknown track alias")
	errMissingPathParameter             = errors.New("missing path parameter")
	errUnexpectedPathParameter          = errors.New("unexpected path parameter on QUIC connection")
	errUnknownTrackStatusRequest        = errors.New("got unexpected track status requrest")
)

type controlMessageStream interface {
	write(wire.ControlMessage) error
	read() iter.Seq2[wire.ControlMessage, error]
}

type objectMessageParser interface {
	Type() wire.StreamType
	Identifier() uint64
	Messages() iter.Seq2[*wire.ObjectMessage, error]
}

// A Session is an endpoint of a MoQ Session session.
type Session struct {
	// Handler
	Handler Handler

	// SubscribeHandler is Handler for Subscribe messages
	SubscribeHandler SubscribeHandler

	// SubscribeUpdateHandler is Handler for SubscribeUpdate messages
	SubscribeUpdateHandler SubscribeUpdateHandler

	// QLOG Logger
	Qlogger *qlog.Logger

	eg              *errgroup.Group
	ctx             context.Context
	cancelCtx       context.CancelFunc
	handshakeDoneCh chan struct{}
	handshakeDone   atomic.Bool

	logger *slog.Logger

	conn          Connection
	controlStream controlMessageStream

	version wire.Version
	path    string

	requestIDs *requestIDGenerator

	trackAliases *sequence
	remoteTracks *remoteTrackMap
	localTracks  *localTrackMap

	outgoingTrackStatusRequests *trackStatusRequestMap
}

func (s *Session) Run(conn Connection) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.eg, s.ctx = errgroup.WithContext(ctx)
	s.cancelCtx = cancel

	var cs Stream
	var err error
	if conn.Perspective() == PerspectiveServer {
		cs, err = conn.AcceptStream(ctx)
	} else if conn.Perspective() == PerspectiveClient {
		cs, err = conn.OpenStreamSync(ctx)
	} else {
		return errors.New("invalid perspective")
	}
	if err != nil {
		return err
	}

	s.handshakeDoneCh = make(chan struct{})
	s.logger = defaultLogger.With("perspective", conn.Perspective())
	s.conn = conn
	s.requestIDs = newRequestIDGenerator(uint64(conn.Perspective()), 0 /*max*/, 2 /*step*/)
	s.trackAliases = newSequence(0, 1)
	s.remoteTracks = newRemoteTrackMap()
	s.localTracks = newLocalTrackMap()
	s.outgoingTrackStatusRequests = newTrackStatusRequestMap()
	s.controlStream = &controlStream{
		stream:  cs,
		logger:  defaultLogger.With("perspective", conn.Perspective()),
		qlogger: nil,
	}

	s.eg.Go(s.readControlStream)
	s.eg.Go(func() error { return s.readStreams(s.ctx) })
	s.eg.Go(func() error { return s.readDatagrams(s.ctx) })

	if s.conn.Perspective() == PerspectiveClient {
		if err := s.sendClientSetup(); err != nil {
			return err
		}
	}
	select {
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	case <-s.handshakeDoneCh:
	}
	return nil
}

func (s *Session) Close() error {
	s.cancelCtx()
	if err := s.conn.CloseWithError(0, ""); err != nil {
		s.logger.Error("failed to close connection", "err", err)
	}
	return s.eg.Wait()
}

func (s *Session) readControlStream() error {
	for msg, err := range s.controlStream.read() {
		if err != nil {
			return err
		}
		if err = s.receive(msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) readStreams(ctx context.Context) error {
	for {
		stream, err := s.conn.AcceptUniStream(ctx)
		if err != nil {
			return err
		}
		// TODO: Instead of starting a goroutine here, start it in the remote
		// stream and close all remote streams when the sesssion closes.
		go func() {
			s.logger.Info("handling new uni stream")
			parser, err := wire.NewObjectStreamParser(stream, stream.StreamID(), s.Qlogger)
			if err != nil {
				return
			}
			s.logger.Debug("parsed object stream header")
			if err := s.handleUniStream(parser); err != nil {
				return
			}
		}()
	}
}

func (s *Session) readDatagrams(ctx context.Context) error {
	for {
		dgram, err := s.conn.ReceiveDatagram(ctx)
		if err != nil {
			return err
		}
		msg := new(wire.ObjectDatagramMessage)
		if _, err = msg.Parse(dgram); err != nil {
			return err
		}
		if s.Qlogger != nil {
			eth := slices.Collect(slices.Map(
				msg.ObjectExtensionHeaders,
				func(e wire.KeyValuePair) moqt.ExtensionHeader {
					return moqt.ExtensionHeader{
						HeaderType:   0, // TODO
						HeaderValue:  0, // TODO
						HeaderLength: 0, // TODO
						Payload:      qlog.RawInfo{},
					}
				}),
			)
			s.Qlogger.Log(moqt.ObjectDatagramEvent{
				EventName:              moqt.ObjectDatagramEventparsed,
				TrackAlias:             msg.TrackAlias,
				GroupID:                msg.GroupID,
				ObjectID:               msg.ObjectID,
				PublisherPriority:      msg.PublisherPriority,
				ExtensionHeadersLength: uint64(len(msg.ObjectExtensionHeaders)),
				ExtensionHeaders:       eth,
				ObjectStatus:           uint64(msg.ObjectStatus),
				Payload: qlog.RawInfo{
					Length:        uint64(len(msg.ObjectPayload)),
					PayloadLength: uint64(len(msg.ObjectPayload)),
					Data:          msg.ObjectPayload,
				},
			})
		}
		if err := s.receiveDatagram(msg); err != nil {
			return err
		}
	}
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
		ForwardingPreference: ObjectForwardingPreferenceDatagram,
		Payload:              msg.ObjectPayload,
	})
	return nil
}

func (s *Session) addLocalTrack(lt *localTrack) error {
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

func (s *Session) getRequestID() (uint64, error) {
	requestID, err := s.requestIDs.next()
	return requestID, err
}

// Path returns the path of the MoQ session which was exchanged during the
// handshake when using QUIC.
func (s *Session) Path() string {
	return s.path
}

// SubscribeOption is a functional option for configuring Subscribe requests.
type SubscribeOption func(*SubscribeOptions)

// WithSubscriberPriority sets the delivery priority for the subscription.
// Priority range is 0-255, with lower values indicating higher priority (0 is highest).
// Default is 128.
func WithSubscriberPriority(priority uint8) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.SubscriberPriority = priority
	}
}

// WithSubscribeGroupOrder sets the group ordering preference for the subscription.
// Default is GroupOrderAscending.
func WithSubscribeGroupOrder(groupOrder GroupOrder) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.GroupOrder = groupOrder
	}
}

// WithForward sets the forward preference for the subscription.
// When true, indicates forward preference. Default is true.
func WithForward(forward bool) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.Forward = forward
	}
}

// WithFilterType sets the subscription filter type.
// Default is FilterTypeLatestObject.
func WithFilterType(filterType FilterType) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.FilterType = filterType
	}
}

// WithStartLocation sets the start position for absolute filters.
// Default is Location{Group: 0, Object: 0}.
func WithStartLocation(location Location) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.StartLocation = location
	}
}

// WithEndGroup sets the end group for range filters.
// Default is 0.
func WithEndGroup(endGroup uint64) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.EndGroup = endGroup
	}
}

// WithAuthorizationToken sets the authorization token for the subscription.
// This is a convenience method that adds the authorization token to parameters.
func WithAuthorizationToken(token string) SubscribeOption {
	return func(opts *SubscribeOptions) {
		if len(token) > 0 {
			// Replace existing auth token or add new one
			for i, param := range opts.Parameters {
				if param.Type == wire.AuthorizationTokenParameterKey {
					opts.Parameters[i].ValueBytes = []byte(token)
					return
				}
			}
			// Add new auth token
			opts.Parameters = append(opts.Parameters, KeyValuePair{
				Type:       wire.AuthorizationTokenParameterKey,
				ValueBytes: []byte(token),
			})
		}
	}
}

// WithSubscribeParameters sets additional key-value parameters for the subscription.
// This replaces any existing parameters.
func WithSubscribeParameters(parameters KVPList) SubscribeOption {
	return func(opts *SubscribeOptions) {
		opts.Parameters = parameters
	}
}

// SubscribeUpdateOption is a functional option for configuring SUBSCRIBE_UPDATE requests.
type SubscribeUpdateOption func(*SubscribeUpdateOptions)

// WithUpdateStartLocation sets the new start position for the subscription update.
// Default is Location{Group: 0, Object: 0}. Note, should not decrease compared
// to the previous start location.
func WithUpdateStartLocation(location Location) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.StartLocation = location
	}
}

// WithUpdateEndGroup sets the new end group for the subscription update.
// EndGroup = 0 means open-ended (no end group limit). Default is 0.
func WithUpdateEndGroup(endGroup uint64) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.EndGroup = endGroup
	}
}

// WithUpdateSubscriberPriority sets the new delivery priority for the subscription update.
// Priority range is 0-255, with lower values indicating higher priority (0 is highest).
// Default is 128.
func WithUpdateSubscriberPriority(priority uint8) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.SubscriberPriority = priority
	}
}

// WithUpdateForward sets the new forward preference for the subscription update.
// When true, indicates forward preference. Default is true.
func WithUpdateForward(forward bool) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.Forward = forward
	}
}

// WithUpdateParameters sets additional key-value parameters for the subscription update.
// This replaces any existing parameters.
func WithUpdateParameters(parameters KVPList) SubscribeUpdateOption {
	return func(opts *SubscribeUpdateOptions) {
		opts.Parameters = parameters
	}
}

// Session message senders

func (s *Session) sendClientSetup() error {
	params := wire.KVPList{}
	if s.conn.Protocol() == ProtocolQUIC {
		path := s.path
		params = append(params, wire.KeyValuePair{
			Type:       wire.PathParameterKey,
			ValueBytes: []byte(path),
		})
	}
	return s.controlStream.write(&wire.ClientSetupMessage{
		SupportedVersions: wire.SupportedVersions,
		SetupParameters:   params,
	})
}

// Subscribe subscribes to a track with the given options.
// It blocks until a response from the peer was received or ctx is cancelled.
//
// Default behavior when no options are provided:
//   - SubscriberPriority: 128 (medium priority)
//   - GroupOrder: GroupOrderAscending
//   - Forward: true (forward preference)
//   - FilterType: FilterTypeLatestObject
//   - StartLocation: Location{Group: 0, Object: 0}
//   - EndGroup: 0
//   - Parameters: empty
//
// Use WithAuthorizationToken(auth) to add authorization.
// Note: auth should not be a simple string, but a structured object containing
// an optional session-specific alias (draft-11 8.2.1.1)
func (s *Session) Subscribe(
	ctx context.Context,
	namespace []string,
	name string,
	options ...SubscribeOption,
) (*RemoteTrack, error) {

	requestID, err := s.getRequestID()
	if err != nil {
		return nil, err
	}
	rt := newRemoteTrack(requestID, func() error {
		return s.unsubscribe(requestID)
	}, func(ctx context.Context, options ...SubscribeUpdateOption) error {
		return s.UpdateSubscription(ctx, requestID, options...)
	})
	if err = s.remoteTracks.addPendingWithAlias(requestID, rt); err != nil {
		return nil, err
	}

	// Set default values
	opts := &SubscribeOptions{
		SubscriberPriority: 128,
		GroupOrder:         GroupOrderAscending,
		Forward:            true,
		FilterType:         FilterTypeLatestObject,
		StartLocation:      Location{Group: 0, Object: 0},
		EndGroup:           0,
		Parameters:         KVPList{},
	}

	// Apply options
	for _, option := range options {
		option(opts)
	}

	cm := &wire.SubscribeMessage{
		RequestID:          requestID,
		TrackNamespace:     namespace,
		TrackName:          []byte(name),
		SubscriberPriority: opts.SubscriberPriority,
		GroupOrder:         opts.GroupOrder,
		Forward:            boolToUint8(opts.Forward),
		FilterType:         opts.FilterType,
		StartLocation:      opts.StartLocation,
		EndGroup:           opts.EndGroup,
		Parameters:         opts.Parameters.ToWire(),
	}
	if err = s.controlStream.write(cm); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		err = context.Cause(ctx)
	case err = <-rt.responseChan:
	}
	if err != nil {
		s.remoteTracks.reject(requestID)
		return nil, err
	}
	return rt, nil
}

// UpdateSubscription sends a SUBSCRIBE_UPDATE message to update an existing subscription.
// No response is expected according to draft-11 specification.
//
// Default behavior when no options are provided:
//   - StartLocation: Location{Group: 0, Object: 0}
//   - EndGroup: 0 (open-ended, no end group limit)
//   - SubscriberPriority: 128 (medium priority)
//   - Forward: true (forward preference)
//   - Parameters: empty
func (s *Session) UpdateSubscription(ctx context.Context, requestID uint64, options ...SubscribeUpdateOption) error {
	// Validate that the subscription exists
	if _, exists := s.remoteTracks.findByRequestID(requestID); !exists {
		return errUnknownRequestID
	}

	// Set default values
	opts := &SubscribeUpdateOptions{
		StartLocation: Location{
			Group:  0,
			Object: 0,
		},
		EndGroup:           0,
		SubscriberPriority: 128,
		Forward:            true,
		Parameters:         KVPList{},
	}

	// Apply options
	for _, option := range options {
		option(opts)
	}

	// Create and send SUBSCRIBE_UPDATE message
	cm := &wire.SubscribeUpdateMessage{
		RequestID:          requestID,
		StartLocation:      opts.StartLocation,
		EndGroup:           opts.EndGroup,
		SubscriberPriority: opts.SubscriberPriority,
		Forward:            boolToUint8(opts.Forward),
		Parameters:         opts.Parameters.ToWire(),
	}

	return s.controlStream.write(cm)
}

// acceptSubscriptionWithOptions accepts a subscription with relevant options.
func (s *Session) acceptSubscriptionWithOptions(id uint64, opts *SubscribeOkOptions) error {
	_, ok := s.localTracks.confirm(id)
	if !ok {
		return errUnknownRequestID
	}

	// Use defaults if opts is nil
	if opts == nil {
		opts = &SubscribeOkOptions{
			Expires:         0,
			GroupOrder:      GroupOrderAscending,
			ContentExists:   false,
			LargestLocation: nil,
			Parameters:      KVPList{},
		}
	}

	msg := &wire.SubscribeOkMessage{
		RequestID:     id,
		Expires:       opts.Expires,
		GroupOrder:    uint8(opts.GroupOrder),
		ContentExists: opts.ContentExists,
		Parameters:    opts.Parameters.ToWire(),
	}

	// Set largest location if content exists and location is provided
	if opts.ContentExists && opts.LargestLocation != nil {
		msg.LargestLocation = *opts.LargestLocation
	}

	return s.controlStream.write(msg)
}

func (s *Session) rejectSubscription(id uint64, errorCode ErrorCodeSubscribe, reason string) error {
	lt, ok := s.localTracks.reject(id)
	if !ok {
		return errUnknownRequestID
	}
	return s.controlStream.write(&wire.SubscribeErrorMessage{
		RequestID:    lt.requestID,
		ErrorCode:    uint64(errorCode),
		ReasonPhrase: reason,
	})
}

func (s *Session) unsubscribe(id uint64) error {
	return s.controlStream.write(&wire.UnsubscribeMessage{
		RequestID: id,
	})
}

func (s *Session) subscriptionDone(id, code, count uint64, reason string) error {
	lt, ok := s.localTracks.delete(id)
	if !ok {
		return errUnknownRequestID
	}
	return s.controlStream.write(&wire.SubscribeDoneMessage{
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
	requestID, err := s.getRequestID()
	if err != nil {
		return nil, err
	}
	rt := newRemoteTrack(requestID, func() error {
		return s.fetchCancel(requestID)
	}, nil)
	if err = s.remoteTracks.addPending(requestID, rt); err != nil {
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
	if err = s.controlStream.write(cm); err != nil {
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
	return s.controlStream.write(&wire.FetchOkMessage{
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
	return s.controlStream.write(&wire.FetchErrorMessage{
		RequestID:    lt.requestID,
		ErrorCode:    errorCode,
		ReasonPhrase: reason,
	})

}

func (s *Session) fetchCancel(id uint64) error {
	return s.controlStream.write(&wire.FetchCancelMessage{
		RequestID: id,
	})
}

func (s *Session) RequestTrackStatus(ctx context.Context, namespace []string, track string) (*TrackStatus, error) {
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
	if err := s.controlStream.write(tsrm); err != nil {
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
	return s.controlStream.write(&wire.TrackStatusMessage{
		StatusCode:      ts.StatusCode,
		RequestID:       0,
		LargestLocation: wire.Location{},
		Parameters:      wire.KVPList{},
	})
}

// Session message handlers

func (s *Session) receive(msg wire.ControlMessage) error {
	s.logger.Info("received message", "type", msg.Type().String(), "msg", msg)

	if !s.handshakeDone.Load() {
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
		s.onGoAway(m)
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
	default:
		err = errUnexpectedMessageType
	}
	return err
}

func (s *Session) onClientSetup(m *wire.ClientSetupMessage) error {
	if s.conn.Perspective() != PerspectiveServer {
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

	path, err := validatePathParameter(m.SetupParameters, s.conn.Protocol() == ProtocolQUIC)
	if err != nil {
		return err
	}
	s.path = path

	if err := s.controlStream.write(&wire.ServerSetupMessage{
		SelectedVersion: wire.Version(selectedVersion),
		SetupParameters: wire.KVPList{},
	}); err != nil {
		return err
	}
	close(s.handshakeDoneCh)
	s.handshakeDone.Store(true)
	return nil
}

func (s *Session) onServerSetup(m *wire.ServerSetupMessage) (err error) {
	if s.conn.Perspective() != PerspectiveClient {
		return errServerReceveidServerSetup
	}

	if !slices.Contains(wire.SupportedVersions, m.SelectedVersion) {
		return errIncompatibleVersions
	}
	s.version = m.SelectedVersion
	close(s.handshakeDoneCh)
	s.handshakeDone.Store(true)
	return nil
}

func (s *Session) onGoAway(msg *wire.GoAwayMessage) {
	s.Handler.Handle(nil, &Message{
		Method:        MessageGoAway,
		NewSessionURI: msg.NewSessionURI,
	})
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

	if len(msg.TrackNamespace) == 0 || len(msg.TrackNamespace) > 32 {
		return errInvalidNamespaceLength
	}
	m := &SubscribeMessage{
		RequestID:          msg.RequestID,
		Namespace:          msg.TrackNamespace,
		Track:              string(msg.TrackName),
		Authorization:      auth,
		SubscriberPriority: msg.SubscriberPriority,
		GroupOrder:         msg.GroupOrder,
		Forward:            msg.Forward,
		FilterType:         msg.FilterType,
		StartLocation:      nil,
		EndGroup:           nil,
		Parameters:         FromWire(msg.Parameters),
	}
	lt := newLocalTrack(s.conn, m.RequestID, s.trackAliases.next(), func(code, count uint64, reason string) error {
		return s.subscriptionDone(m.RequestID, code, count, reason)
	}, s.Qlogger)

	if err := s.addLocalTrack(lt); err != nil {
		code := ErrorCodeInternal
		reason := "internal"
		if err == errMaxRequestIDViolated {
			code = ErrorCodeTooManyRequests
			reason = "too many subscribes"
		}
		return s.controlStream.write(&wire.SubscribeErrorMessage{
			RequestID:    lt.requestID,
			ErrorCode:    uint64(code),
			ReasonPhrase: reason,
		})
	}
	srw := &SubscribeResponseWriter{
		id:         m.RequestID,
		trackAlias: lt.trackAlias,
		session:    s,
		localTrack: lt,
		handled:    false,
	}
	if s.SubscribeHandler != nil {
		s.SubscribeHandler.HandleSubscribe(srw, m)
	}
	if !srw.handled {
		if s.SubscribeHandler == nil {
			s.logger.Warn("no SubscribeHandler set, rejecting subscription",
				"request_id", m.RequestID, "track_alias", m.TrackAlias)
		}
		return srw.Reject(0, "unhandled subscription")
	}
	return nil
}

func (s *Session) onSubscribeOk(msg *wire.SubscribeOkMessage) error {
	rt, err := s.remoteTracks.confirm(msg.RequestID)
	if err != nil {
		return err
	}
	if err = s.remoteTracks.setAlias(msg.RequestID, msg.TrackAlias); err != nil {
		// TODO: Protocol violation
		return err
	}

	// Store complete subscription information from SUBSCRIBE_OK
	rt.expires = msg.Expires
	rt.groupOrder = GroupOrder(msg.GroupOrder)
	rt.contentExists = msg.ContentExists
	if rt.contentExists {
		rt.largestLocation = &msg.LargestLocation
	} else {
		rt.largestLocation = nil
	}
	rt.parameters = FromWire(msg.Parameters)

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
	err := ProtocolError{
		code:    ErrorCode(msg.ErrorCode),
		message: msg.ReasonPhrase,
	}
	select {
	case sub.responseChan <- err:
	default:
		s.logger.Info("dropping unhandled SubscribeError response")
	}
	return nil
}

func (s *Session) onSubscribeUpdate(msg *wire.SubscribeUpdateMessage) error {
	// Find the local track for this request ID to validate it exists
	_, ok := s.localTracks.findByID(msg.RequestID)
	if !ok {
		// According to draft-11, should close session with Protocol Violation
		// if Request ID doesn't exist
		return errUnknownRequestID
	}

	// Convert wire message to public message struct
	publicMsg := &SubscribeUpdateMessage{
		RequestID:          msg.RequestID,
		StartLocation:      msg.StartLocation,
		EndGroup:           msg.EndGroup,
		SubscriberPriority: msg.SubscriberPriority,
		Forward:            msg.Forward,
		Parameters:         FromWire(msg.Parameters),
	}

	// Propagate to application handler if available
	if s.SubscribeUpdateHandler != nil {
		s.SubscribeUpdateHandler.HandleSubscribeUpdate(publicMsg)
	}

	// For now, accept the update without enforcing constraints
	// A full implementation would validate narrowing constraints per draft-11
	return nil
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
	if len(msg.TrackNamespace) == 0 || len(msg.TrackNamespace) > 32 {
		return errInvalidNamespaceLength
	}
	m := &Message{
		Method:        MessageFetch,
		Namespace:     msg.TrackNamespace,
		Track:         string(msg.TrackName),
		RequestID:     msg.RequestID,
		Authorization: "",
		NewSessionURI: "",
		ErrorCode:     0,
		ReasonPhrase:  "",
	}
	lt := newLocalTrack(s.conn, m.RequestID, s.trackAliases.next(), nil, s.Qlogger)
	if err := s.addLocalTrack(lt); err != nil {
		if rejectErr := s.rejectFetch(m.RequestID, uint64(ErrorCodeSubscribeInternal), ""); rejectErr != nil {
			return rejectErr
		}
		return err
	}
	frw := &fetchResponseWriter{
		id:         m.RequestID,
		session:    s,
		localTrack: lt,
		handled:    false,
	}
	s.Handler.Handle(frw, m)
	if !frw.handled {
		return frw.Reject(0, "unhandled fetch")
	}
	return nil
}

func (s *Session) onFetchOk(msg *wire.FetchOkMessage) error {
	rt, err := s.remoteTracks.confirm(msg.RequestID)
	if err != nil {
		return err
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
		code:    ErrorCode(msg.ErrorCode),
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
	if len(msg.TrackNamespace) == 0 || len(msg.TrackNamespace) > 32 {
		return errInvalidNamespaceLength
	}
	tsrw := &trackStatusResponseWriter{
		session: s,
		handled: false,
		status: TrackStatus{
			Namespace:    msg.TrackNamespace,
			Trackname:    string(msg.TrackName),
			StatusCode:   0,
			LastGroupID:  0,
			LastObjectID: 0,
		},
	}
	s.Handler.Handle(tsrw, &Message{
		Method:    MessageTrackStatusRequest,
		Namespace: msg.TrackNamespace,
		Track:     string(msg.TrackName),
	})
	if !tsrw.handled {
		return tsrw.Reject(0, "")
	}
	return nil
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

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
