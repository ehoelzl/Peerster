package types

import (
	"net"
)

func (gp *Gossiper) AddNode(udpAddr *net.UDPAddr) {
	// Checks if the given `*net.UDPAddr` is in `gp.Nodes`, if not, it adds it.
	for _, nodeAddr := range gp.Nodes {
		if nodeAddr.String() == udpAddr.String() {
			return
		}
	}
	gp.Nodes = append(gp.Nodes, udpAddr)
}

func (gp *Gossiper) DeleteTicker(node *net.UDPAddr) {
	if elem, ok := gp.Tickers[node.String()]; ok {
		close(elem)
		delete(gp.Tickers, node.String())
	}
}

func (gp *Gossiper) AddRumorMessage(rumor *RumorMessage) bool {
	// Tries to add the rumor message to its list, returns true if it was added
	elem, ok := gp.Peers[rumor.Origin]
	messageAdded := false
	if ok { //Means we already have this peer in memory
		messageAdded = elem.AddMessage(rumor) // Only adds the message if NextID matches
	} else {
		gp.Peers[rumor.Origin] = NewPeer(rumor.Origin)
		messageAdded = gp.Peers[rumor.Origin].AddMessage(rumor)
	}
	return messageAdded
}

func (gp *Gossiper) IsInSync(status *StatusPacket) bool {
	for _, s := range status.Want { // First check if we have all messages they have
		if _, ok := gp.Peers[s.Identifier]; !ok {
			gp.Peers[s.Identifier] = NewPeer(s.Identifier)
		}
		if gp.Peers[s.Identifier].NextID != s.NextID {
			return false
		}
	}

	for _, peer := range gp.Peers { // Then check if they have all messages we have
		found := false
		for _, s := range status.Want {
			if s.Identifier == peer.Identifier {
				found = true
				if s.NextID != peer.NextID { // not in sync
					return false
				}
			}
		}
		if !found && peer.NextID > 1 { // They have a missing peer
			return false
		}
	}
	return true
}

func (gp *Gossiper) CheckTickers(sender *net.UDPAddr, status *StatusPacket) bool{
	// Checks the tickers for the given sender and status Message and stops all tickers that have been acked
	if _, ok := gp.Tickers[sender.String()]; ok {
		gp.DeleteTicker(sender)
		return true
	}
	return false
}