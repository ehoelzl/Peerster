package types

import (
	"log"
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

func (gp *Gossiper) DeleteTicker(node *net.UDPAddr, origin string, messageId uint32) {
	if e, ok := gp.Tickers[node.String()]; ok {
		if e1, ok1 := e[origin]; ok1 {
			if e2, ok2 := e1.Tickers[messageId]; ok2 {
				//fmt.Printf("Deleted ticker for %v origin %v message id %v \n", node.String(), origin, messageId)

				e2 <- true
				delete(gp.Tickers[node.String()][origin].Tickers, messageId) // Delete ticker
				if len(gp.Tickers[node.String()][origin].Tickers) == 0 {     // If no more ticker, delete
					delete(gp.Tickers[node.String()], origin)
				}
				if len(gp.Tickers[node.String()]) == 0 {
					delete(gp.Tickers, node.String())
				}
			}
		}
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
		for pName, peer := range gp.Peers {
			if pName == s.Identifier && peer.NextID != s.NextID {
				return false
			}
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

func (gp *Gossiper) CheckTickers(sender *net.UDPAddr, status *StatusPacket) {
	log.Printf("%v\n", gp.Tickers)
	// Checks the tickers for the given sender and status Message and stops all tickers that have been acked
	if elem, ok := gp.Tickers[sender.String()]; ok {
		for _, s := range status.Want { // Iterate on all status messages
			if elem2, ok2 := elem[s.Identifier]; ok2 { //Map for origin
				for mid, _ := range elem2.Tickers {
					if mid < s.NextID { // Means other node has this message id hence delete ticker
						gp.DeleteTicker(sender, s.Identifier, mid) //
					}
				}
			}
		}
	}
}