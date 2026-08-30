package moqtransport

import (
	"errors"
)

// maxPendingObjects is the number of objects buffered per track alias that is
// not bound to a receiver yet.
// TODO: Make configurable.
const maxPendingObjects = 100

// maxPendingTracks is the number of unbound track aliases that may buffer
// objects at the same time.
// TODO: Make configurable.
const maxPendingTracks = 16

var errDuplicateTrackAlias = errors.New("track alias already in use")

type objectReceiver interface {
	push(*Object)
}

// trackEntry routes the objects of one track alias. Objects that arrive before
// the alias is bound to a receiver are buffered in pending.
type trackEntry struct {
	receiver objectReceiver
	pending  []*Object
}

// pushObject delivers o to the receiver of trackAlias. The track alias is
// assigned by the peer in SUBSCRIBE_OK, so objects can arrive before it is
// known. They are buffered until the alias is bound to a receiver.
func (s *Session) pushObject(trackAlias uint64, o *Object) {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()

	entry, ok := s.tracks[trackAlias]
	if ok && entry.receiver != nil {
		entry.receiver.push(o)
		return
	}
	// TODO: Decide whether exceeding a limit should close the session with a
	// protocol violation instead of dropping the object.
	if !ok && s.pendingTracks >= maxPendingTracks {
		s.logger.Info("too many unbound track aliases: dropping incoming object", "trackAlias", trackAlias)
		return
	}
	if ok && len(entry.pending) >= maxPendingObjects {
		s.logger.Info("pending object buffer overflow: dropping incoming object", "trackAlias", trackAlias)
		return
	}
	if !ok {
		entry = &trackEntry{}
		s.tracks[trackAlias] = entry
		s.pendingTracks++
	}
	entry.pending = append(entry.pending, o)
}

// bindTrackAlias attaches r to trackAlias and hands it the objects that
// arrived before the alias was known.
func (s *Session) bindTrackAlias(trackAlias uint64, r objectReceiver) error {
	s.tracksLock.Lock()
	defer s.tracksLock.Unlock()

	entry, ok := s.tracks[trackAlias]
	if !ok {
		s.tracks[trackAlias] = &trackEntry{receiver: r}
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
