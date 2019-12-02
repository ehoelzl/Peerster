package types

import (
	"sync"
)

/*PeerRumors is an encapsulation of all the messages with one origin (the peer)*/
type PeerRumors struct {
	nextId      uint32
	Rumors      map[uint32]*RumorMessage
	TLCMessages map[uint32]*TLCMessage
}

func newPeerRumors() *PeerRumors {
	return &PeerRumors{
		nextId:      1,
		Rumors:      make(map[uint32]*RumorMessage),
		TLCMessages: make(map[uint32]*TLCMessage),
	}
}

func (p *PeerRumors) addRumor(rumor *RumorMessage) bool {
	/*Adds the given rumor to the struct, and increments nextId accordingly (accepts rumors when ID >= nextID*/
	isNewRumor := false
	if _, ok := p.Rumors[rumor.ID]; rumor.ID >= p.nextId && !ok { // New message
		isNewRumor = true
		p.Rumors[rumor.ID] = rumor // Add new message to list
		if rumor.ID == p.nextId {  // Update
			p.nextId += 1
		}
		p.fixNextId()
	}
	return isNewRumor
}

func (p *PeerRumors) fixNextId() {
	_, hasRumor := p.Rumors[p.nextId]
	_, hasTLC := p.TLCMessages[p.nextId]
	for ; hasRumor || hasTLC; { // Means the Next ID should be incremented
		p.nextId += 1
		_, hasRumor = p.Rumors[p.nextId]
		_, hasTLC = p.TLCMessages[p.nextId]
	}
}

func (p *PeerRumors) addTLCMessage(tlc *TLCMessage) bool {
	/*Adds the given rumor to the struct, and increments nextId accordingly (accepts rumors when ID >= nextID*/
	isNewTLC := false
	if _, ok := p.TLCMessages[tlc.ID]; tlc.ID >= p.nextId && !ok { // New message
		isNewTLC = true
		p.TLCMessages[tlc.ID] = tlc // Add new TLCMessage to list
		if tlc.ID == p.nextId {     // Update
			p.nextId += 1
		}
		p.fixNextId()
	}
	return isNewTLC
}

func (p *PeerRumors) getPacketAt(index uint32) *GossipPacket {
	if rumor, ok := p.Rumors[index]; ok {
		return &GossipPacket{Rumor: rumor}
	} else if tlc, ok := p.TLCMessages[index]; ok {
		return &GossipPacket{TLCMessage: tlc}
	}
	return nil
}

/*======================= Definition of A Collection of peers =======================*/
/*Data structure that encapsulates all the RumorMessages and their origin*/
type Rumors struct {
	rumors map[string]*PeerRumors
	sync.RWMutex
}

func InitRumorStruct(identifier string) *Rumors {
	/*Initializes the RumorsStruct given the name of the running gossiper*/
	initialPeer := newPeerRumors()
	rumors := make(map[string]*PeerRumors)
	rumors[identifier] = initialPeer

	return &Rumors{
		rumors: rumors,
	}
}

func (r *Rumors) CreateNewRumor(origin string, message string) *RumorMessage {
	/*Creates the next message for the given origin (only to be used when sending chat message or rumor message from calling gossiper)*/
	r.RLock()
	defer r.RUnlock()
	rumor := &RumorMessage{
		Origin: origin,
		ID:     r.rumors[origin].nextId,
		Text:   message,
	}
	return rumor
}

func (r *Rumors) CreateNewTLCMessage(origin string, confirmed int, txBlock BlockPublish, withStatus bool) *TLCMessage {
	r.RLock()
	defer r.RUnlock()
	var vectorClock *StatusPacket
	if withStatus {
		vectorClock = r.getStatusPacket()
	}
	tlc := &TLCMessage{
		Origin:      origin,
		ID:          r.rumors[origin].nextId,
		Confirmed:   confirmed,
		TxBlock:     txBlock,
		VectorClock: vectorClock,
	}
	return tlc
}

func (r *Rumors) AddRumorMessage(rumor *RumorMessage) bool {
	/*Adds the given rumor message to the structure, and returns a bool to check if it was a newRumpor (i.e. it was added)*/
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

func (r *Rumors) AddTLCMessage(tlc *TLCMessage) bool {
	r.Lock()
	defer r.Unlock()
	isNewTLC := false
	if elem, ok := r.rumors[tlc.Origin]; ok {
		isNewTLC = elem.addTLCMessage(tlc)
	} else {
		r.rumors[tlc.Origin] = newPeerRumors()
		isNewTLC = r.rumors[tlc.Origin].addTLCMessage(tlc)
	}
	return isNewTLC
}

func (r *Rumors) getStatusPacket() *StatusPacket {
	var statusMessages []PeerStatus
	for pName, p := range r.rumors {
		statusMessages = append(statusMessages, PeerStatus{Identifier: pName, NextID: p.nextId})
	}
	return &StatusPacket{Want: statusMessages}
}

func (r *Rumors) GetStatusPacket() *StatusPacket {
	/*Constructs and returns the current status*/
	r.RLock()
	defer r.RUnlock()
	statusPacket := r.getStatusPacket()
	return statusPacket
}

func (r *Rumors) GetTLCMessageBlock(name string, uid uint32) (*BlockPublish, bool) {
	r.RLock()
	defer r.RUnlock()
	if peer, ok := r.rumors[name]; ok {
		if tlc, ok := peer.TLCMessages[uid]; ok {
			return &tlc.TxBlock, true
		}
	}
	return nil, false

}

func (r *Rumors) CompareStatus(statusPacket *StatusPacket) (*GossipPacket, bool, bool) {
	/*Compares the given status to the struct, and returns the missingMessage, a flag indicating whether they are missing messages, and a flag if I am missing messages*/
	r.RLock()
	defer r.RUnlock()
	statusMap := statusPacket.ToMap()

	// First check if they have missing messages
	for identifier, peer := range r.rumors {
		nextId, ok := statusMap[identifier]
		if !ok && peer.nextId > 1 { // The peer is not present in their status, and has new messages
			packet := peer.getPacketAt(1)

			return packet, true, false // Send pack first message
		} else if ok && nextId < peer.nextId { // They have missing messages from this peer
			packet := peer.getPacketAt(nextId) // Select the missing message

			return packet, true, false
		}
	}

	// Then check if I have missing messages
	for _, status := range statusPacket.Want {
		elem, ok := r.rumors[status.Identifier]
		if !ok { // If peer not present locally, check if they have at least one message
			if status.NextID > 1 {
				return nil, false, true
			}
		} else if status.NextID > elem.nextId { //check if their NextId is bigger then mine
			return nil, false, true
		}
	}
	return nil, false, false
}

func (r *Rumors) GetAll() map[string]*PeerRumors {
	/*Returns all the rumors in the struct*/
	r.RLock()
	defer r.RUnlock()
	return r.rumors
}
