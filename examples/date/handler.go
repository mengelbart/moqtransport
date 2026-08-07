package main

import (
	"log"
	"strings"

	"github.com/mengelbart/moqtransport"
)

type handler struct {
	endpoint  *endpoint
	sessionID uint64
}

func (h *handler) HandleGoAway(newSessionURI string) {
	log.Printf("sessionNr: %d got goaway, new session URI: %s", h.sessionID, newSessionURI)
}

func (h *handler) HandleSubscribe(r *moqtransport.IncomingSubscribeRequest) {
	log.Printf("got subscribe request, sessionID: %d", h.sessionID)
	ns := tupleToStringList(r.Namespace())
	if !h.endpoint.publish {
		log.Printf("sessionNr: %d got unexpected subscribe request: %v", h.sessionID, ns)
		r.Reject(moqtransport.SubscribeErrorCodeTrackDoesNotExist, "endpoint does not publish any tracks") //nolint:errcheck
		return
	}
	if !tupleEqual(ns, h.endpoint.namespace) || string(r.Name()) != h.endpoint.trackname {
		log.Printf("got unexpected subscribe namespace/track: %v/%v, expected %v/%v", ns, r.Name(), h.endpoint.namespace, h.endpoint.trackname)
		r.Reject(moqtransport.SubscribeErrorCodeTrackDoesNotExist, "unknown track")
		return
	}
	// TODO: Set track alias
	r.Accept(0)

	log.Printf("sessionNr: %d accepted subscription for %v--%v", h.sessionID, strings.Join(ns, "/"), string(r.Name()))
	h.endpoint.lock.Lock()
	h.endpoint.publishers[r] = struct{}{}
	h.endpoint.lock.Unlock()
}

func tupleToStringList(tuple [][]byte) []string {
	list := []string{}
	for _, t := range tuple {
		list = append(list, string(t))
	}
	return list
}
