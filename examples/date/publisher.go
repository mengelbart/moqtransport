package main

import (
	"log"

	"github.com/mengelbart/moqtransport"
)

type publisher struct {
	p           moqtransport.Publisher
	session     *moqtransport.Session
	sessionID   uint64
	subscribeID uint64
	trackAlias  uint64
}

func (p *publisher) SendDatagram(o moqtransport.Object) error {
	return p.p.SendDatagram(o)
}

func (p *publisher) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*moqtransport.Subgroup, error) {
	log.Printf("sessionNr: %d, subscribeID: %d, trackAlias: %d, groupID: %d, subgroupID: %v",
		p.sessionID, p.subscribeID, p.trackAlias, groupID, subgroupID)
	return p.p.OpenSubgroup(groupID, subgroupID, priority)
}

func (p *publisher) CloseWithError(code uint64, reason string) error {
	return p.p.CloseWithError(code, reason)
}
