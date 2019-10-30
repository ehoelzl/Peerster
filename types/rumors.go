package types

import (
	"sync"
)

/*PeerRumors is an encapsulation of all the messages with one origin (the peer)*/
type PeerRumors struct {
	nextId   uint32
	Messages map[uint32]*RumorMessage
}

func newPeerRumors() *PeerRumors {
	return &PeerRumors{
		nextId:   1,
		Messages: make(map[uint32]*RumorMessage),
	}
}

func (p *PeerRumors) addRumor(rumor *RumorMessage) bool {
	// Adds the given message to the peer, checks if rumor.ID == p.NextId
	isNewRumor := false
	if rumor.ID >= p.nextId { // New message
		isNewRumor = true
		p.Messages[rumor.ID] = rumor // Add new message to list
		if rumor.ID == p.nextId {    // Update
			p.nextId += 1
		}
		// Now see if we should increment next ID (received later messages)
		for _, ok := p.Messages[p.nextId]; ok; { // Means the Next ID should be incremented
			p.nextId +=1
			_, ok = p.Messages[p.nextId]
		}
	}
	return isNewRumor
}

/*======================= Definition of A Collection of peers =======================*/
/*Data structure that encapsulates all the RumorMessages and their origin*/
type Rumors struct {
	rumors map[string]*PeerRumors
	sync.RWMutex
}

func NewRumorStruct(identifier string) *Rumors {
	//Creates the GossipPeers structure, and adds the identifier of the current gossiper
	initialPeer := newPeerRumors()
	rumors := make(map[string]*PeerRumors)
	rumors[identifier] = initialPeer

	return &Rumors{
		rumors: rumors,
	}
}

func (r *Rumors) CreateNewRumor(origin string, message string) *RumorMessage {
	//Creates the next message for the given origin (only to be used when sending chat message or rumor message from calling gossiper)
	r.RLock()
	defer r.RUnlock()
	rumor := &RumorMessage{
		Origin: origin,
		ID:     r.rumors[origin].nextId,
		Text:   message,
	}
	return rumor
}

func (r *Rumors) AddRumorMessage(rumor *RumorMessage) bool {
	r.Lock()
	defer r.Unlock()
	isNewRumor := false

	if elem, ok := r.rumors[rumor.Origin]; ok { // Check if we already have the Origin
		isNewRumor = elem.addRumor(rumor) // If yes, add message
	} else {
		r.rumors[rumor.Origin] = newPeerRumors()
		isNewRumor = r.rumors[rumor.Origin].addRumor(rumor)

	}
	return isNewRumor
}

func (r *Rumors) GetStatusPacket() *StatusPacket {
	r.RLock()
	defer r.RUnlock()
	var statusMessages []PeerStatus
	for pName, p := range r.rumors {
		statusMessages = append(statusMessages, PeerStatus{Identifier: pName, NextID: p.nextId})
	}
	return &StatusPacket{Want: statusMessages}
}

func (r *Rumors) CompareStatus(statusPacket *StatusPacket) (*RumorMessage, bool, bool) {
	// Returns (missingMessage, isMissingMessage, amMissingMessage)
	r.RLock()
	defer r.RUnlock()
	statusMap := statusPacket.ToMap()

	var message *RumorMessage

	// First check if they have missing messages
	for identifier, peer := range r.rumors {
		nextId, ok := statusMap[identifier]
		if !ok && peer.nextId > 1{ // The peer is not present in their status, and has new messages
			message = peer.Messages[1]
			return message, true, false // Send pack first message
		} else if ok && nextId < peer.nextId { // They have missing messages from this peer
			message = peer.Messages[nextId] // Select the missing message
			return message, true, false
		}
	}

	// Then check if I have missing messages
	for _, status := range statusPacket.Want {
		elem, ok := r.rumors[status.Identifier]
		if !ok { // If peer not present locally, check if they have at least one message
			if status.NextID > 1{
				return nil, false, true
			}
		} else if status.NextID > elem.nextId { //check if their NextId is bigger then mine
			return nil, false, true
		}
	}
	return nil, false, false
}

func (r *Rumors) GetAll() map[string]*PeerRumors {
	//Function only for server
	r.RLock()
	defer r.RUnlock()
	return r.rumors
}