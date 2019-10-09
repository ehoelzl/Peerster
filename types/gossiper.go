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
	Tickers       map[string]map[string][]*utils.PeerTicker // nodeAddress -> originName -> Ticker
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
		Tickers:       make(map[string]map[string][]*utils.PeerTicker),
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
			go gp.simpleBroadcast(packet, nil) // Set `OriginalName` to the node's name
		} else {
			// Create RumorPacket, store it to keep track (for current Node)
			// Select Random Peer and send rumor to it
			message := &RumorMessage{
				Origin: gp.Name,
				ID:     gp.Peers[gp.Name].NextID,
				Text:   packet.Contents,
			}
			messageAdded, _ := gp.Peers[gp.Name].AddMessage(message)
			if messageAdded { //Rumor Only if message was added (i.e. never seen before and is coherent with NextID)
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

				messageAdded, isAhead := gp.AddRumorMessage(rumor) // Add message to list
				if isAhead || messageAdded {
					go gp.SendStatusMessage(udpAddr) // Both cases need to send status
					if messageAdded {
						go gp.StartRumormongering(rumor, map[string]struct{}{udpAddr.String(): struct{}{}}, false) // Monger with other nodes except this one
					} else {
						fmt.Printf("Message %v not added\n", &rumor)
					}
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
					gp.CheckMissingMessages(status, udpAddr)
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

func (gp *Gossiper) AddRumorMessage(rumor *RumorMessage) (bool, bool) {
	// First bool says if the message was added, second to check if it is ahead of the current node
	elem, ok := gp.Peers[rumor.Origin]
	messageAdded, isAhead := false, false
	if ok { //Means we already have this peer in memory
		messageAdded, isAhead = elem.AddMessage(rumor) // Only adds the message if NextID matches
	} else {
		gp.Peers[rumor.Origin] = NewPeer(rumor.Origin)
		messageAdded, isAhead = gp.Peers[rumor.Origin].AddMessage(rumor)
	}
	return messageAdded, isAhead
}

func (gp *Gossiper) IsInSync(status *StatusPacket) bool {
	for _, s := range status.Want {
		elem, ok := gp.Peers[s.Identifier]
		if !ok {
			gp.Peers[s.Identifier] = NewPeer(s.Identifier) // In that case, create new Peer
			if s.NextID > 1 {                              // Means this node already has messages
				return false
			}
		} else {                         // Peer exists in local, need to check NextID
			if elem.NextID != s.NextID { //
				return false
			}
		}

	}
	return true
}

func (gp *Gossiper) CheckMissingMessages(status *StatusPacket, other *net.UDPAddr) {
	for _, ps := range status.Want {
		elem, ok := gp.Peers[ps.Identifier]  // Should always be OK
		if ok && (elem.NextID > ps.NextID) { // Current gossiper has messages that other doesn't
			nextMessage := elem.Messages[ps.NextID] // Send the message that other wants
			go gp.SendRumorMessage(nextMessage, other, nil)
		}
	}

	// Now check if current gossiper has missing messages
}

func (gp *Gossiper) CheckTickers(sender *net.UDPAddr, status *StatusPacket) bool {
	/* Checks the tickers for the given sender and status Message*/
	tickerStopped := false
	if elem, ok := gp.Tickers[sender.String()]; ok {
		for _, s := range status.Want { // Iterate on all status messages
			if elem2, ok2 := elem[s.Identifier]; ok2 { // Means there are tickers for this Identifier
				var remaining []*utils.PeerTicker
				for _, t := range elem2 { // Iterate over all tickers for this origin, sender pair
					if s.NextID > t.ID { // Means sender has gotten the message
						t.AckReceived <- true
						tickerStopped = true
					} else {
						remaining = append(remaining, t) // Keep remaining tickers (message was not delivered yet)
					}
				}
				gp.Tickers[sender.String()][s.Identifier] = remaining
			}
		}
	}
	return tickerStopped
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
		go gp.SendRumorMessage(message, randomNode, func() { go gp.StartRumormongering(message, except, false) })
	}
}

func (gp *Gossiper) SendRumorMessage(message *RumorMessage, to *net.UDPAddr, callback func()) {
	gp.SendPacket(nil, message, nil, to)
	// Start ticker for this node, origin name and message ID
	if _, ok := gp.Tickers[to.String()]; !ok {
		gp.Tickers[to.String()] = make(map[string][]*utils.PeerTicker)
	}
	elem := gp.Tickers[to.String()][message.Origin]
	gp.Tickers[to.String()][message.Origin] = append(elem, utils.NewPeerTimer(message.ID, callback, 10))
}

func (gp *Gossiper) SendStatusMessage(to *net.UDPAddr) {
	var statusMessages []PeerStatus
	for _, p := range gp.Peers {
		statusMessages = append(statusMessages, PeerStatus{Identifier: p.Identifier, NextID: p.NextID,})
	}
	statusPacket := &StatusPacket{Want: statusMessages}
	gp.SendPacket(nil, nil, statusPacket, to)
}
