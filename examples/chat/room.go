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
	track   *moqtransport.ListTrack
	session *moqtransport.Session
}

type room struct {
	ID           roomID
	catalogTrack *moqtransport.ListTrack
	catalogGroup uint64
	users        *chatalog[*user]
	usersLock    sync.Mutex
}

func newRoom(id roomID) *room {
	return &room{
		ID:           id,
		catalogTrack: moqtransport.NewListTrack(),
		catalogGroup: 0,
		users:        &chatalog[*user]{version: 1, participants: map[string]*user{}},
		usersLock:    sync.Mutex{},
	}
}

func (r *room) addParticipant(username string, session *moqtransport.Session, track *moqtransport.ListTrack) error {
	r.usersLock.Lock()
	defer r.usersLock.Unlock()
	log.Printf("saving user: %v", username)
	if _, ok := r.users.participants[username]; ok {
		return errors.New("duplicate participant")
	}
	for _, u := range r.users.participants {
		if err := session.AddLocalTrack(fmt.Sprintf("moq-chat/%v/participant/%v", r.ID, u.name), "", u.track); err != nil {
			panic(err)
		}
		if err := u.session.AddLocalTrack(fmt.Sprintf("moq-chat/%v/participant/%v", r.ID, username), "", track); err != nil {
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
	sub, err := s.Subscribe(context.Background(), 0, 0, [][]byte{[]byte(fmt.Sprintf("moq-chat/%v/participant/%v", r.ID, username))}, []byte(""), "")
	if err != nil {
		panic(err)
	}
	catalog := r.users.serialize()
	fmt.Printf("sending catalog: %v\n", catalog)
	r.catalogTrack.Append(moqtransport.Object{
		GroupID:              r.catalogGroup,
		ObjectID:             0,
		PublisherPriority:    0,
		ForwardingPreference: moqtransport.ObjectForwardingPreferenceStreamTrack,
		Payload:              []byte(catalog),
	})
	r.catalogGroup += 1
	go func(remote *moqtransport.RemoteTrack, local *moqtransport.ListTrack) {
		for {
			obj, err := remote.ReadObject(context.Background())
			if err != nil {
				panic(err)
			}
			fmt.Printf("relay read object: %v\n", obj)
			local.Append(obj)
		}
	}(sub, u.track)
}

func (r *room) subscribeCatalog(s *moqtransport.Session, sub *moqtransport.Subscription, srw moqtransport.SubscriptionResponseWriter) {
	if err := s.AddLocalTrack(fmt.Sprintf("moq-chat/%v", r.ID), "", r.catalogTrack); err != nil {
		srw.Reject(uint64(errorCodeInternal), "failed to setup room catalog track")
		return
	}
	track := moqtransport.NewListTrack()
	err := r.addParticipant(sub.Authorization, s, track)
	if err != nil {
		srw.Reject(uint64(errorCodeDuplicateUsername), "username already in use")
		return
	}
	srw.Accept(r.catalogTrack)
}
