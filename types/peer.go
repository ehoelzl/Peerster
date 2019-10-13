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

func (p *Peer) addMessage(rumor *RumorMessage) bool {
	if rumor.ID != p.NextID {
		return false
	} else {
		p.Messages[rumor.ID] = rumor
		p.NextID += 1
		return true
	}
}

/*======================= Definition of A Collection of peers =======================*/

type GossipPeers struct {
	Peers map[string]*Peer
	Lock  sync.RWMutex
}

func NewPeers() *GossipPeers {
	return &GossipPeers{
		Peers: make(map[string]*Peer),
		Lock:  sync.RWMutex{},
	}
}

func (peers *GossipPeers) AddPeer(identifier string) {
	// Assumes this peer does not exist and Lock was
	peers.Lock.Lock()
	defer peers.Lock.Unlock()
	peers.Peers[identifier] = newPeer()
}

func (peers *GossipPeers) AddRumorMessage(rumor *RumorMessage) bool {
	peers.Lock.Lock()
	defer peers.Lock.Unlock()
	messageAdded := false
	if elem, ok := peers.Peers[rumor.Origin]; ok {
		messageAdded = elem.addMessage(rumor)
	} else {
		peers.Peers[rumor.Origin] = newPeer()
		messageAdded = peers.Peers[rumor.Origin].addMessage(rumor)
	}
	return messageAdded
}

func (peers *GossipPeers) GetStatusMessage() []PeerStatus {
	peers.Lock.RLock()
	defer peers.Lock.RUnlock()
	var statusMessages []PeerStatus
	for pName, p := range peers.Peers {
		statusMessages = append(statusMessages, PeerStatus{Identifier: pName, NextID: p.NextID,})
	}
	return statusMessages
}

func (peers *GossipPeers) NextId(identifier string) uint32 {
	peers.Lock.RLock()
	defer peers.Lock.RUnlock()
	return peers.Peers[identifier].NextID
}

func (peers *GossipPeers) GetTheirMissingMessage(statusPacket *StatusPacket) *RumorMessage {
	peers.Lock.RLock()
	defer peers.Lock.RUnlock()
	statusMap := statusPacket.ToMap()

	for identifier, peer := range peers.Peers {
		if nextId, ok := statusMap[identifier]; ok {
			if nextId < peer.NextID { // Peer is missing messages from peer.Identifier
				message := peers.Peers[identifier].Messages[nextId] // Get the missing message
				return message
			}
		} else if peer.NextID > 1 { // Peer not present and has messages
			message := peers.Peers[identifier].Messages[1] // Get the first message
			return message
		}
	}
	return nil
}

func (peers *GossipPeers) IsMissingMessage(status *StatusPacket) bool {
	peers.Lock.Lock()
	defer peers.Lock.Unlock()

	for _, status := range status.Want {
		if _, ok := peers.Peers[status.Identifier]; !ok { // If peer not present, create new peer
			peers.Peers[status.Identifier] = newPeer()
		}
		elem := peers.Peers[status.Identifier]
		if elem.NextID < status.NextID {
			return true
		}
	}
	return false
}