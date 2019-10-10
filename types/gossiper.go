package types

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/ehoelzl/Peerster/utils"
	"log"
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
	Tickers       map[string]chan bool // nodeAddress -> messageId -> bool
	Buffer        map[string]*RumorMessage // Keeps track of the last rumor sent to this node
}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool) *Gossiper {
	// Creates new gossiper with the given parameters
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
		Tickers:       make(map[string]chan bool),
		Buffer:        make(map[string]*RumorMessage),
	}
}

func (gp *Gossiper) StartClientListener() {
	// Listener for client
	packetBytes := make([]byte, 1024)
	packet := &SimpleMessage{}
	for {
		// First read packet
		n, _, err := gp.ClientConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from client Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from client Port for %v\n", gp.Name))

		fmt.Println("CLIENT MESSAGE", packet.Contents)

		// Start mongering or broadcasting
		if gp.IsSimple {
			packet.RelayPeerAddr = gp.GossipAddress.String()
			packet.OriginalName = gp.Name
			go gp.simpleBroadcast(packet, nil)
		} else {

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
	// Gossip listener
	packetBytes := make([]byte, 1024)
	packet := &GossipPacket{}
	for {
		// Read packet from other gossipers (always RumorMessage)
		n, udpAddr, err := gp.GossipConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from gossip Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from gossip Port for %v\n", gp.Name))

		gp.AddNode(udpAddr) // Check if new node
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
			} else if packet.Status != nil { // Status Message
				status := packet.Status
				status.PrintStatusMessage(udpAddr)
				utils.PrintAddresses(gp.Nodes)

				gp.CheckTickers(udpAddr, status)
				if gp.IsInSync(status) {
					fmt.Printf("IN SYNC WITH %v\n", udpAddr.String())
					//fmt.Println(gp.Tickers)
					flip := utils.CoinFlip()
					if flip {
						except := map[string]struct{}{udpAddr.String(): struct{}{}}
						/*log.Println(gp.Buffer)
						log.Println(udpAddr.String())*/
						go gp.StartRumormongering(gp.Buffer[udpAddr.String()], except, true)
					}
				} else {
					gp.CheckMissingMessages(status, udpAddr)
				}
			} else {
				fmt.Printf("Packet from %v is empty\n", udpAddr.String())
			}
		}
	}
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
					log.Printf("Peer %v is missing message %v from %v\n", other.String(), ps.NextID, ps.Identifier)
					return
				}
			}
		}
		if !found { // Means peer.Identifier not present in status message
			//fmt.Printf("Peer %v not found\n", peer.Identifier)
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

/*-------------------- Methods used for transferring messages and broadcasting ------------------------------*/

func (gp *Gossiper) SendStatusMessage(to *net.UDPAddr) {
	// Creates the vector clock for the given peer
	var statusMessages []PeerStatus
	for _, p := range gp.Peers {
		statusMessages = append(statusMessages, PeerStatus{Identifier: p.Identifier, NextID: p.NextID,})
	}
	statusPacket := &StatusPacket{Want: statusMessages}
	gp.SendPacket(nil, nil, statusPacket, to)
}

func (gp *Gossiper) SendPacket(simple *SimpleMessage, rumor *RumorMessage, status *StatusPacket, to *net.UDPAddr) {
	gossipPacket, err := protobuf.Encode(&GossipPacket{Simple: simple, Rumor: rumor, Status: status})
	utils.CheckError(err, fmt.Sprintf("Error encoding gossipPacket for %v\n", gp.Nodes))
	_, err = gp.GossipConn.WriteToUDP(gossipPacket, to)
	utils.CheckError(err, fmt.Sprintf("Error sending gossipPacket from node %v to node %v\n", gp.GossipAddress.String(), to.String()))
}

func (gp *Gossiper) simpleBroadcast(packet *SimpleMessage, except *net.UDPAddr) {
	// Double functionality: Broadcasts to all peers if except == nil, or to all except the given one
	for _, nodeAddr := range gp.Nodes {
		if nodeAddr.String() != except.String() {
			gp.SendPacket(packet, nil, nil, nodeAddr)
		}
	}
}

func (gp *Gossiper) SendRumorMessage(message *RumorMessage, to *net.UDPAddr, callback func()) {
	// Sends the message to the given node, and creates a timer
	gp.SendPacket(nil, message, nil, to)

	// Then start ticker for this node/origin/messageId
	gp.DeleteTicker(to) // Check if ticker already exists for this message

	gp.Tickers[to.String()] = utils.NewPeerTimer(to, callback, 10)
	gp.Buffer[to.String()] = message
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
			go gp.StartRumormongering(message, except, false)       // Rumormonger with other nodes
			gp.DeleteTicker(randomNode) // Delete ticker
		}
		gp.SendRumorMessage(message, randomNode, callback)
	}
}
