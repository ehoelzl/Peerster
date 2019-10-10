package types

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/ehoelzl/Peerster/utils"
	"net"
)

type Gossiper struct {
	ClientAddress *net.UDPAddr
	ClientConn    *net.UDPConn
	GossipAddress *net.UDPAddr
	GossipConn    *net.UDPConn
	Name          string
	Nodes         []*net.UDPAddr
	Peers         map[string]*Peer // From name to peer
	IsSimple      bool
	Tickers       map[string]map[string]*utils.PeerTickers // nodeAddress -> messageId -> bool

}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool) *Gossiper {
	clientAddr, err := net.ResolveUDPAddr("udp4", uiAddress)
	clientConn, err := net.ListenUDP("udp4", clientAddr) // Connection to client
	utils.CheckError(err, fmt.Sprintf("Error when opening client UDP channel for %v\n", name))

	gossipAddr, err := net.ResolveUDPAddr("udp4", gossipAddress)
	gossipConn, err := net.ListenUDP("udp4", gossipAddr)
	utils.CheckError(err, fmt.Sprintf("Error when opening gossip UDP channel for %v\n", name))
	fmt.Printf("Starting gossiper %v\n UIAddress: %v\n GossipAddress %v\n Peers %v\n\n", name, clientAddr, gossipAddr, initialPeers)

	selfPeer := NewPeer(name)
	peers := map[string]*Peer{name: selfPeer}
	return &Gossiper{
		ClientAddress: clientAddr,
		ClientConn:    clientConn,
		GossipAddress: gossipAddr,
		GossipConn:    gossipConn,
		Name:          name,
		Nodes:         utils.ParseAddresses(initialPeers),
		Peers:         peers,
		IsSimple:      simple,
		Tickers:       make(map[string]map[string]*utils.PeerTickers),
	}
}

func (gp *Gossiper) StartClientListener() {
	packetBytes := make([]byte, 1024)
	packet := &SimpleMessage{}
	for {
		n, _, err := gp.ClientConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from client Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from client Port for %v\n", gp.Name))

		fmt.Println("CLIENT MESSAGE", packet.Contents)
		if gp.IsSimple {
			packet.RelayPeerAddr = gp.GossipAddress.String()
			packet.OriginalName = gp.Name
			go gp.simpleBroadcast(packet, nil)
		} else {
			// Create RumorPacket, store it to keep track (for current Node)
			// Select Random Peer and send rumor to it
			message := &RumorMessage{
				Origin: gp.Name,
				ID:     gp.Peers[gp.Name].NextID,
				Text:   packet.Contents,
			}
			messageAdded := gp.Peers[gp.Name].AddMessage(message) // Usually, message is always added
			if messageAdded {                                     //Rumor Only if message was added (i.e. never seen before and is coherent with NextID)
				go gp.StartRumormongering(message, nil, false)
			}
		}
	}

}

func (gp *Gossiper) StartGossipListener() {
	packetBytes := make([]byte, 1024)
	packet := &GossipPacket{}
	for {
		n, udpAddr, err := gp.GossipConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from gossip Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from gossip Port for %v\n", gp.Name))

		gp.checkNewNode(udpAddr) // Check if new node
		// Start rumormongering
		if gp.IsSimple {
			fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v \n", packet.Simple.OriginalName, packet.Simple.RelayPeerAddr, packet.Simple.Contents)
			utils.PrintAddresses(gp.Nodes)

			packet.Simple.RelayPeerAddr = gp.GossipAddress.String()
			go gp.simpleBroadcast(packet.Simple, udpAddr) // keep original name
		} else {
			if packet.Rumor != nil { // Rumor message
				rumor := packet.Rumor
				fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n", rumor.Origin, udpAddr.String(), rumor.ID, rumor.Text)
				utils.PrintAddresses(gp.Nodes)

				messageAdded := gp.AddRumorMessage(rumor) // Add message to list
				go gp.SendStatusMessage(udpAddr)
				if messageAdded {
					except := map[string]struct{}{udpAddr.String(): struct{}{}}
					go gp.StartRumormongering(rumor, except, false) // Monger with other nodes except this one
				}
			} else { // Status Message
				status := packet.Status
				status.PrintStatusMessage(udpAddr)
				utils.PrintAddresses(gp.Nodes)

				gp.CheckTickers(udpAddr, status)
				if gp.IsInSync(status) {
					fmt.Printf("IN SYNC WITH %v\n", udpAddr.String())
					// Coin flip
				} else {
					go gp.CheckMissingMessages(status, udpAddr)
				}

			}
		}
	}
}

func (gp *Gossiper) checkNewNode(udpAddr *net.UDPAddr) {
	// Checks if the given `*net.UDPAddr` is in `gp.Nodes`, if not, it adds it.
	for _, nodeAddr := range gp.Nodes {
		if nodeAddr.String() == udpAddr.String() {
			return
		}
	}
	gp.Nodes = append(gp.Nodes, udpAddr)
}

func (gp *Gossiper) deleteTicker(node *net.UDPAddr, origin string, messageId uint32) {
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
	// First bool says if the message was added, second to check if it is ahead of the current node
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

func (gp *Gossiper) CheckMissingMessages(status *StatusPacket, other *net.UDPAddr) {
	// First check if they have missing messages
	for _, peer := range gp.Peers {
		found := false
		for _, ps := range status.Want {
			if ps.Identifier == peer.Identifier {
				found = true
				if ps.NextID < peer.NextID { // We have messages that other peer doesn't
					message := gp.Peers[peer.Identifier].Messages[ps.NextID] // Get the missing message
					go gp.SendRumorMessage(message, other, nil)
					return
				}
			}
		}
		if !found { // Means peer.Identifier not present in status message
			fmt.Printf("Peer %v not found\n", peer.Identifier)
			message := gp.Peers[peer.Identifier].Messages[1] // Get the first message
			go gp.SendRumorMessage(message, other, nil)
			return
		}
	}

	// Then check if I have missing messages
	for _, ps := range status.Want {
		if _, ok := gp.Peers[ps.Identifier]; !ok { // If peer not present, create new peer
			gp.Peers[ps.Identifier] = NewPeer(ps.Identifier)
		}
		elem := gp.Peers[ps.Identifier]
		if elem.NextID < ps.NextID {
			go gp.SendStatusMessage(other)
			return
		}
	}
}

func (gp *Gossiper) CheckTickers(sender *net.UDPAddr, status *StatusPacket) {
	// Checks the tickers for the given sender and status Message and stops all tickers that have been acked
	if elem, ok := gp.Tickers[sender.String()]; ok {
		for _, s := range status.Want { // Iterate on all status messages
			if elem2, ok2 := elem[s.Identifier]; ok2 { //Map for origin
				for mid, _ := range elem2.Tickers {
					if mid < s.NextID { // Means other node has this message id hence delete ticker
						gp.deleteTicker(sender, s.Identifier, mid) //
					}
				}
			}
		}
	}
}

/*-------------------- Methods used for transferring messages and broadcasting ------------------------------*/
func (gp *Gossiper) simpleBroadcast(packet *SimpleMessage, except *net.UDPAddr) {
	// Double functionality: Broadcasts to all peers if except == nil, or to all except the given one
	for _, nodeAddr := range gp.Nodes {
		if nodeAddr.String() != except.String() {
			gp.SendPacket(packet, nil, nil, nodeAddr)
		}
	}
}

func (gp *Gossiper) SendPacket(simple *SimpleMessage, rumor *RumorMessage, status *StatusPacket, to *net.UDPAddr) {
	gossipPacket, err := protobuf.Encode(&GossipPacket{Simple: simple, Rumor: rumor, Status: status})
	utils.CheckError(err, fmt.Sprintf("Error encoding gossipPacket for %v\n", gp.Nodes))
	_, err = gp.GossipConn.WriteToUDP(gossipPacket, to)
	utils.CheckError(err, fmt.Sprintf("Error sending gossipPacket from node %v to node %v\n", gp.GossipAddress.String(), to.String()))
}

func (gp *Gossiper) StartRumormongering(message *RumorMessage, except map[string]struct{}, coinFlip bool) {
	// Picks random node and sends RumorMessage
	randomNode := utils.RandomNode(gp.Nodes, except) // Pick random node
	if randomNode != nil {
		if coinFlip {
			fmt.Printf("FLIPPED COIN sending rumor to %v\n", randomNode.String())
		} else {
			fmt.Printf("MONGERING with %v\n", randomNode.String())
		}
		if except == nil {
			except = make(map[string]struct{})
		}
		except[randomNode.String()] = struct{}{} // Add the receiver node to `except` list to avoid loops

		callback := func() {
			gp.deleteTicker(randomNode, message.Origin, message.ID) // Delete ticker
			go gp.StartRumormongering(message, except, false)       // Rumormonger with other nods
		}
		gp.SendRumorMessage(message, randomNode, callback)
	}
}

func (gp *Gossiper) SendRumorMessage(message *RumorMessage, to *net.UDPAddr, callback func()) {
	// Sends the message to the given node, and creates a timer
	gp.SendPacket(nil, message, nil, to)

	// Then start ticker for this node/origin/messageId
	gp.deleteTicker(to, message.Origin, message.ID) // Check if ticker already exists for this message
	if _, ok := gp.Tickers[to.String()]; !ok {
		gp.Tickers[to.String()] = make(map[string]*utils.PeerTickers) // If nothing for this node, create
	}
	if _, ok := gp.Tickers[to.String()][message.Origin]; !ok {
		gp.Tickers[to.String()][message.Origin] = &utils.PeerTickers{Tickers: make(map[uint32]chan bool)}
	}

	elem := gp.Tickers[to.String()][message.Origin]
	gp.Tickers[to.String()][message.Origin] = utils.NewPeerTimer(message.ID, elem, callback, 10)
}

func (gp *Gossiper) SendStatusMessage(to *net.UDPAddr) {
	// Creates the vector clock for the given peer
	var statusMessages []PeerStatus
	for _, p := range gp.Peers {
		statusMessages = append(statusMessages, PeerStatus{Identifier: p.Identifier, NextID: p.NextID,})
	}
	statusPacket := &StatusPacket{Want: statusMessages}
	gp.SendPacket(nil, nil, statusPacket, to)
}
