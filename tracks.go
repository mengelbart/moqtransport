package moqtransport

import (
	"context"
	"errors"
)

var (
	errDuplicateTrackAlias  = errors.New("track alias already in use")
	errTooManyPendingTracks = errors.New("too many unbound track aliases")
)

type objectReceiver interface {
	push(*Object)
}

type trackEntry struct {
	receiver objectReceiver
	pending  []*Object
	bound    chan struct{}
}

func newTrackEntry() *trackEntry {
	return &trackEntry{
		bound: make(chan struct{}),
	}
}

func (s *Session) pushStreamObject(trackAlias uint64, o *Object) error {
	receiver, err := s.waitForReceiver(trackAlias)
	if err != nil {
		return err
	}
	receiver.push(o)
	return nil
}

func (s *Session) pushDatagramObject(trackAlias uint64, o *Object) {
	s.tracksLock.Lock()

	entry, ok := s.tracks[trackAlias]
	if ok && entry.receiver != nil {
		entry.receiver.push(o)
		s.tracksLock.Unlock()
		return
	}
	if !ok {
		if s.pendingTracks >= s.maxPendingTracks {
			s.tracksLock.Unlock()
			s.closeTooManyPendingTracks()
			return
		}
		entry = newTrackEntry()
		s.tracks[trackAlias] = entry
		s.pendingTracks++
	}
	if len(entry.pending) >= s.maxPendingObjects {
		s.tracksLock.Unlock()
		s.logger.Info("pending object buffer overflow: dropping incoming object", "trackAlias", trackAlias)
		return
	}
	entry.pending = append(entry.pending, o)
	s.tracksLock.Unlock()
}

func (s *Session) waitForReceiver(trackAlias uint64) (objectReceiver, error) {
	s.tracksLock.Lock()
	entry, ok := s.tracks[trackAlias]
	if ok && entry.receiver != nil {
		s.tracksLock.Unlock()
		return entry.receiver, nil
	}
	if !ok {
		if s.pendingTracks >= s.maxPendingTracks {
			s.tracksLock.Unlock()
			s.closeTooManyPendingTracks()
			return nil, errTooManyPendingTracks
		}
		entry = newTrackEntry()
		s.tracks[trackAlias] = entry
		s.pendingTracks++
	}
	bound := entry.bound
	s.tracksLock.Unlock()

	select {
	case <-bound:
	case <-s.ctx.Done():
		return nil, context.Cause(s.ctx)
	}

	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()
	return entry.receiver, nil
}

func (s *Session) closeTooManyPendingTracks() {
	s.closeWithError(&SessionError{
		Code:   uint64(ErrorCodeInternal),
		Reason: errTooManyPendingTracks.Error(),
	})
}

func (s *Session) bindTrackAlias(trackAlias uint64, r objectReceiver) error {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()

	entry, ok := s.tracks[trackAlias]
	if !ok {
		entry = newTrackEntry()
		entry.receiver = r
		close(entry.bound)
		s.tracks[trackAlias] = entry
		return nil
	}
	if entry.receiver != nil {
		return errDuplicateTrackAlias
	}
	entry.receiver = r
	s.pendingTracks--
	for _, o := range entry.pending {
		r.push(o)
	}
	entry.pending = nil
	close(entry.bound)
	return nil
}

// removeReceiver drops the track alias entry of r, if it has one.
func (s *Session) removeReceiver(r objectReceiver) {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()

	for trackAlias, entry := range s.tracks {
		if entry.receiver == r {
			delete(s.tracks, trackAlias)
			return
		}
	}
}
