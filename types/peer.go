package types

import (
	"sync"
)

/*======================= Definition of One Peer =======================*/
type Peer struct {
	ID       int
	NextID   uint32
	Messages map[uint32]*RumorMessage
}

func newPeer(id int) *Peer {
	return &Peer{
		ID:       id,
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
	Status []PeerStatus
	Peers  map[string]*Peer
	Lock   sync.RWMutex
}

func NewPeers() *GossipPeers {
	return &GossipPeers{
		Status: nil,
		Peers:  make(map[string]*Peer),
		Lock:   sync.RWMutex{},
	}
}

func (peers *GossipPeers) AddPeer(identifier string) {
	// Assumes this peer does not exist and Lock was
	peers.Lock.Lock()
	defer peers.Lock.Unlock()
	peers.Peers[identifier] = newPeer(len(peers.Status))
	peers.Status = append(peers.Status, PeerStatus{
		Identifier: identifier,
		NextID:     1,
	})
}

func (peers *GossipPeers) AddRumorMessage(rumor *RumorMessage) bool {
	peers.Lock.Lock()
	defer peers.Lock.Unlock()
	messageAdded := false

	if elem, ok := peers.Peers[rumor.Origin]; ok {
		messageAdded = elem.addMessage(rumor)
	} else {
		peers.Peers[rumor.Origin] = newPeer(len(peers.Status))
		peers.Status = append(peers.Status, PeerStatus{ // New peer, add to status
			Identifier: rumor.Origin,
			NextID:     1,
		})
		messageAdded = peers.Peers[rumor.Origin].addMessage(rumor)
	}
	if messageAdded {
		peers.Status[peers.Peers[rumor.Origin].ID].NextID += 1 // If message was added, increase status ID
	}
	return messageAdded
}

/*func (peers *GossipPeers) GetStatusMessage() []PeerStatus {
	peers.Lock.RLock()
	defer peers.Lock.RUnlock()
	var statusMessages []PeerStatus
	for _, p := range peers.Peers {
		statusMessages = append(statusMessages, p.Status)
	}
	return peers.Status
}*/

/*func (peers *GossipPeers) NextId(identifier string) uint32 {
	peers.Lock.RLock()
	defer peers.Lock.RUnlock()
	return peers.Peers[identifier].NextID
}*/

func (peers *GossipPeers) CompareStatus(statusPacket *StatusPacket) (*RumorMessage, bool) {
	peers.Lock.RLock()
	defer peers.Lock.RUnlock()
	statusMap := statusPacket.ToMap()

	var message *RumorMessage
	for identifier, peer := range peers.Peers {
		if nextId, ok := statusMap[identifier]; ok {
			if nextId < peer.NextID { // Peer is missing messages from peer.Identifier
				message = peers.Peers[identifier].Messages[nextId] // Get the missing message
				return message, false
			}
		} else if peer.NextID > 1 { // Peer not present and has messages
			message = peers.Peers[identifier].Messages[1] // Get the first message
			return message, false
		}
	}
	for _, status := range statusPacket.Want {
		if _, ok := peers.Peers[status.Identifier]; !ok { // Peer not present
			if status.NextID > 1 {
				return nil, true
			}
		} else {
			elem := peers.Peers[status.Identifier]
			if elem.NextID < status.NextID {
				return nil, true
			}
		}

	}
	return nil, false
}