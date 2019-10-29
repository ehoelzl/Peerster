package types

import (
	"sync"
)

/*======================= Definition of One Peer =======================*/
type Peer struct {
	NextID   uint32
	Messages map[uint32]*RumorMessage
}

func newPeer() *Peer {
	return &Peer{
		NextID:   1,
		Messages: make(map[uint32]*RumorMessage),
	}
}

func (p *Peer) addRumor(rumor *RumorMessage) bool {
	// Adds the given message to the peer, checks if rumor.ID == p.NextId
	isNewRumor := false
	if rumor.ID >= p.NextID { // New message
		isNewRumor = true
		p.Messages[rumor.ID] = rumor // Add new message to list
		if rumor.ID == p.NextID { // Update
			p.NextID += 1
		}
		// Now see if we should increment next ID (received later messages)
		for _, ok := p.Messages[p.NextID]; ok; { // Means the Next ID should be incremented
			p.NextID +=1
			_, ok = p.Messages[p.NextID]
		}
	}
	return isNewRumor
}

/*======================= Definition of A Collection of peers =======================*/

type GossipPeers struct {
	Peers map[string]*Peer
	sync.RWMutex
}

func NewPeers(identifier string) *GossipPeers {
	//Creates the GossipPeers structure, and adds the identifier of the current gossiper
	initialPeer := newPeer()
	peers := make(map[string]*Peer)
	peers[identifier] = initialPeer

	return &GossipPeers{
		Peers: peers,
	}
}

func (peers *GossipPeers) CreateNewMessage(origin string, message string) *RumorMessage {
	//Creates the next message for the given origin (only to be used when sending chat message or rumor message from calling gossiper)
	peers.RLock()
	defer peers.RUnlock()
	rumor := &RumorMessage{
		Origin: origin,
		ID:     peers.Peers[origin].NextID,
		Text:   message,
	}
	return rumor
}

func (peers *GossipPeers) AddRumorMessage(rumor *RumorMessage) bool {
	peers.Lock()
	defer peers.Unlock()
	isNewRumor := false

	if elem, ok := peers.Peers[rumor.Origin]; ok { // Check if we already have the Origin
		isNewRumor = elem.addRumor(rumor) // If yes, add message
	} else {
		peers.Peers[rumor.Origin] = newPeer()
		isNewRumor = peers.Peers[rumor.Origin].addRumor(rumor)

	}
	return isNewRumor
}

func (peers *GossipPeers) GetStatusPacket() *StatusPacket {
	peers.RLock()
	defer peers.RUnlock()
	var statusMessages []PeerStatus
	for pName, p := range peers.Peers {
		statusMessages = append(statusMessages, PeerStatus{Identifier: pName, NextID: p.NextID})
	}
	return &StatusPacket{Want: statusMessages}
}

func (peers *GossipPeers) CompareStatus(statusPacket *StatusPacket) (*RumorMessage, bool, bool) {
	// Returns (missingMessage, isMissingMessage, amMissingMessage)
	peers.RLock()
	defer peers.RUnlock()
	statusMap := statusPacket.ToMap()

	var message *RumorMessage

	// First check if they have missing messages
	for identifier, peer := range peers.Peers {
		nextId, ok := statusMap[identifier]
		if !ok && peer.NextID > 1{ // The peer is not present in their status, and has new messages
			message = peer.Messages[1]
			return message, true, false // Send pack first message
		} else if ok && nextId < peer.NextID{ // They have missing messages from this peer
			message = peer.Messages[nextId] // Select the missing message
			return message, true, false
		}
	}

	// Then check if I have missing messages
	for _, status := range statusPacket.Want {
		elem, ok := peers.Peers[status.Identifier]
		if !ok { // If peer not present locally, check if they have at least one message
			if status.NextID > 1{
				return nil, false, true
			}
		} else {
			return nil, false, status.NextID > elem.NextID // If peer present, check if their NextId is bigger then mine
		}
	}
	return nil, false, false
}
