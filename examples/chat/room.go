package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/mengelbart/moqtransport"
)

type roomID string

type user struct {
	name    string
	track   *moqtransport.LocalTrack
	session *moqtransport.Session
}

type room struct {
	ID           roomID
	catalogTrack *moqtransport.LocalTrack
	catalogGroup uint64
	users        *chatalog[*user]
	usersLock    sync.Mutex
}

func newRoom(id roomID) *room {
	return &room{
		ID:           id,
		catalogTrack: moqtransport.NewLocalTrack(fmt.Sprintf("moq-chat/%v", id), ""),
		catalogGroup: 0,
		users:        &chatalog[*user]{version: 1, participants: map[string]*user{}},
		usersLock:    sync.Mutex{},
	}
}

func (r *room) addParticipant(username string, session *moqtransport.Session, track *moqtransport.LocalTrack) error {
	r.usersLock.Lock()
	defer r.usersLock.Unlock()
	log.Printf("saving user: %v", username)
	if _, ok := r.users.participants[username]; ok {
		return errors.New("duplicate participant")
	}
	for _, u := range r.users.participants {
		if err := session.AddLocalTrack(u.track); err != nil {
			panic(err)
		}
		if err := u.session.AddLocalTrack(track); err != nil {
			panic(err)
		}
	}
	r.users.participants[username] = &user{
		name:    username,
		track:   track,
		session: session,
	}
	return nil
}

func (r *room) findParticipant(username string) (*user, bool) {
	r.usersLock.Lock()
	defer r.usersLock.Unlock()
	log.Printf("finding user: %v", username)
	u, ok := r.users.participants[username]
	return u, ok
}

func (r *room) announceUser(username string, s *moqtransport.Session, arw moqtransport.AnnouncementResponseWriter) {
	u, ok := r.findParticipant(username)
	if !ok {
		arw.Reject(uint64(errorCodeUnknownParticipant), fmt.Sprintf("username '%v' not found, participant must join before announcing", username))
	}
	arw.Accept()
	sub, err := s.Subscribe(context.Background(), 0, 0, fmt.Sprintf("moq-chat/%v/participant/%v", r.ID, username), "", "")
	if err != nil {
		panic(err)
	}
	catalog := r.users.serialize()
	fmt.Printf("sending catalog: %v\n", catalog)
	r.catalogTrack.WriteObject(context.Background(), moqtransport.Object{
		GroupID:              r.catalogGroup,
		ObjectID:             0,
		ObjectSendOrder:      0,
		ForwardingPreference: moqtransport.ObjectForwardingPreferenceStreamTrack,
		Payload:              []byte(catalog),
	})
	r.catalogGroup += 1
	go func(remote *moqtransport.RemoteTrack, local *moqtransport.LocalTrack) {
		for {
			obj, err := remote.ReadObject(context.Background())
			if err != nil {
				panic(err)
			}
			fmt.Printf("relay read object: %v\n", obj)
			err = local.WriteObject(context.Background(), obj)
			if err != nil {
				panic(err)
			}
		}
	}(sub, u.track)
}

func (r *room) subscribeCatalog(s *moqtransport.Session, sub *moqtransport.Subscription, srw moqtransport.SubscriptionResponseWriter) {
	if err := s.AddLocalTrack(r.catalogTrack); err != nil {
		srw.Reject(uint64(errorCodeInternal), "failed to setup room catalog track")
		return
	}
	track := moqtransport.NewLocalTrack(fmt.Sprintf("moq-chat/%v/participant/%v", r.ID, sub.Authorization), "") // TODO: Track ID?
	err := r.addParticipant(sub.Authorization, s, track)
	if err != nil {
		srw.Reject(uint64(errorCodeDuplicateUsername), "username already in use")
		return
	}
	srw.Accept(r.catalogTrack)
}
