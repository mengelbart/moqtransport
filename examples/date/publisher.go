package main

import (
	"log"

	"github.com/mengelbart/moqtransport"
)

type publisher struct {
	p         moqtransport.Publisher
	sessionID uint64
	requestID uint64
}

func (p *publisher) SendDatagram(o moqtransport.Object) error {
	return p.p.SendDatagram(o)
}

func (p *publisher) OpenSubgroup(groupID, subgroupID uint64, priority uint8) (*moqtransport.Subgroup, error) {
	log.Printf("sessionNr: %d, requestID: %d, groupID: %d, subgroupID: %v",
		p.sessionID, p.requestID, groupID, subgroupID)
	return p.p.OpenSubgroup(groupID, subgroupID, priority)
}

func (p *publisher) CloseWithError(code uint64, reason string) error {
	return p.p.CloseWithError(code, reason)
}
