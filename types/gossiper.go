package types

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"net"
	"time"
)

type Gossiper struct {
	ClientAddress *net.UDPAddr
	ClientConn    *net.UDPConn
	GossipAddress *net.UDPAddr
	GossipConn    *net.UDPConn
	IsSimple      bool
	Name          string
	Nodes         *Nodes
	Peers         *GossipPeers // From name to peer
	Routing       *RoutingTable
}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool, antiEntropy uint) (*Gossiper, bool) {
	// Creates new gossiper with the given parameters
	if len(name) == 0 {
		fmt.Println("Please provide a name for the gossiper")
		return nil, false
	}
	clientAddr, err := net.ResolveUDPAddr("udp4", uiAddress)
	utils.CheckFatalError(err, fmt.Sprintf("Could not resolve UI Address %v\n", uiAddress))

	clientConn, err := net.ListenUDP("udp4", clientAddr) // Connection to client
	utils.CheckFatalError(err, fmt.Sprintf("Error when opening client UDP channel for %v\n", name))

	gossipAddr, err := net.ResolveUDPAddr("udp4", gossipAddress)
	utils.CheckFatalError(err, fmt.Sprintf("Could not resolve Gossip Address Address %v\n", gossipAddress))
	gossipConn, err := net.ListenUDP("udp4", gossipAddr)
	utils.CheckFatalError(err, fmt.Sprintf("Error when opening gossip UDP channel for %v\n", name))

	log.Printf("Starting gossiper %v\n UIAddress: %v\n GossipAddress %v\n Peers %v\n\n", name, clientAddr, gossipAddr, initialPeers)

	peers := NewPeers(name) // Create Peer structure for messages
	gossiper := &Gossiper{
		ClientAddress: clientAddr,
		ClientConn:    clientConn,
		GossipAddress: gossipAddr,
		GossipConn:    gossipConn,
		Name:          name,
		IsSimple:      simple,
		Nodes:         NewNodes(initialPeers),
		Peers:         peers,
		Routing:       NewRoutingTable(),
	}

	// AntiEntropy timer
	go utils.NewTicker(func() {
		randomNode, ok := gossiper.Nodes.GetRandom(nil)
		if ok {
			gossiper.SendStatusMessage(randomNode)
		}
	}, time.Duration(antiEntropy))

	return gossiper, true
}

func (gp *Gossiper) HandleClientMessage(packetBytes []byte) {
	message := &Message{}
	err := protobuf.Decode(packetBytes, message)
	if err != nil {
		log.Println("Could not decode packet from client")
		return
	}
	if len(message.Text) == 0 {
		log.Println("Empty message")
		return
	}

	fmt.Printf("CLIENT MESSAGE %v\n", message.Text)

	if gp.IsSimple { // Simple case
		simpleMessage := &SimpleMessage{
			OriginalName:  gp.Name,
			RelayPeerAddr: gp.GossipAddress.String(),
			Contents:      message.Text,
		}
		gp.SimpleBroadcast(simpleMessage, nil)
	} else {
		message := &RumorMessage{
			Origin: gp.Name,
			ID:     gp.Peers.Peers[gp.Name].NextID,
			Text:   message.Text,
		}
		messageAdded := gp.Peers.AddRumorMessage(message) // Usually, message is always added
		if messageAdded {                                 //Rumor Only if message was not seen before (i.e. added)
			gp.StartRumormongering(message, nil, false, true)
		}
	}
}

func (gp *Gossiper) HandleGossipPacket(from *net.UDPAddr, packetBytes []byte) {
	packet := &GossipPacket{}
	err := protobuf.Decode(packetBytes, packet)
	if err != nil {
		log.Println(err)
		log.Printf("Could not decode GossipPacket from %v\n", from.String())
		return
	}

	gp.Nodes.Add(from) // Add node to list if not in
	if gp.IsSimple {
		if packet.Simple == nil {
			log.Printf("Empty simple message from %v\n", from.String())
			return
		}
		fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v \n", packet.Simple.OriginalName, packet.Simple.RelayPeerAddr, packet.Simple.Contents)
		gp.Nodes.Print()

		packet.Simple.RelayPeerAddr = gp.GossipAddress.String()
		gp.SimpleBroadcast(packet.Simple, from)
	} else {
		if rumor := packet.Rumor; rumor != nil { // RumorMessage
			gp.HandleRumorMessage(from, rumor)

		} else if status := packet.Status; status != nil { //StatusPacket
			gp.HandleStatusPacket(from, status)

		} else {
			log.Printf("Empty packet from %v\n", from.String())
			return
		}
	}
}

func (gp *Gossiper) HandleRumorMessage(from *net.UDPAddr, rumor *RumorMessage) {
	// Assumes rumor is not nil
	fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n", rumor.Origin, from.String(), rumor.ID, rumor.Text)
	gp.Nodes.Print()
	messageAdded := gp.Peers.AddRumorMessage(rumor) // Add message to list
	go gp.SendStatusMessage(from)                   // Send back ack

	if messageAdded { // If message was not seen before, continue rumor mongering to other nodes
		gp.Routing.UpdateRoute(rumor.Origin, from.String()) // Update routing table
		except := map[string]struct{}{from.String(): struct{}{}} // Monger with other nodes except this one
		gp.StartRumormongering(rumor, except, false, true)
	}
}

func (gp *Gossiper) HandleStatusPacket(from *net.UDPAddr, status *StatusPacket) {
	status.PrintStatusMessage(from)
	gp.Nodes.Print()
	lastRumor, isAck := gp.Nodes.CheckTimeouts(from)
	missingMessage, isMissing, amMissing := gp.Peers.CompareStatus(status)
	inSync := !(isMissing || amMissing)

	if inSync {
		fmt.Printf("IN SYNC WITH %v\n", from.String())
		if isAck {
			if flip := utils.CoinFlip(); flip {
				except := map[string]struct{}{from.String(): struct{}{}} // Monger with other nodes except this one
				gp.StartRumormongering(lastRumor, except, true, false)   // Start mongering, but no timeout
			}
		}
	} else {
		if isMissing { // Means they are missing a message
			gp.SendPacket(nil, missingMessage, nil, from) // Send back missing message
		} else if amMissing { // Means I am missing a message
			gp.SendStatusMessage(from) // I have missing messages
		}
	}
}

/*-------------------- Methods used for transferring messages and broadcasting ------------------------------*/

func (gp *Gossiper) SendPacket(simple *SimpleMessage, rumor *RumorMessage, status *StatusPacket, to *net.UDPAddr) {
	gossipPacket, err := protobuf.Encode(&GossipPacket{Simple: simple, Rumor: rumor, Status: status})
	if err != nil {
		log.Printf("Error encoding gossipPacket for %v\n", to.String())
		return
	}
	_, err = gp.GossipConn.WriteToUDP(gossipPacket, to)

	if err != nil {
		log.Printf("Error sending gossipPacket to node %v\n", to.String())
	}

}

func (gp *Gossiper) SendStatusMessage(to *net.UDPAddr) {
	// Creates the vector clock for the given peer
	statusPacket := gp.Peers.GetStatusPacket()
	gp.SendPacket(nil, nil, statusPacket, to)
}

func (gp *Gossiper) SimpleBroadcast(packet *SimpleMessage, except *net.UDPAddr) {
	// Double functionality: Broadcasts to all peers if except == nil, or to all except the given one
	gp.Nodes.RLock()
	defer gp.Nodes.RUnlock()

	for nodeAddr, node := range gp.Nodes.Addresses {
		if nodeAddr != except.String() {
			gp.SendPacket(packet, nil, nil, node.udpAddr)
		}
	}
}

func (gp *Gossiper) StartRumormongering(message *RumorMessage, except map[string]struct{}, coinFlip bool, withTimeout bool) {
	// Picks random receiver and Mongers the message
	randomNode, ok := gp.Nodes.GetRandom(except)
	if !ok {
		return
	}
	if coinFlip {
		fmt.Printf("FLIPPED COIN sending rumor to %v\n", randomNode.String())
	} else {
		fmt.Printf("MONGERING with %v\n", randomNode.String())
	}

	if except == nil {
		except = make(map[string]struct{})
	}
	except[randomNode.String()] = struct{}{} // Add node we send to, to the except list

	gp.SendPacket(nil, message, nil, randomNode) // Send rumor
	if withTimeout {
		callback := func() {
			fmt.Printf("Timeout on message %v sent to %v\n", message, randomNode)
			go gp.StartRumormongering(message, except, false, true) // Monger with other node
			gp.Nodes.DeleteTicker(randomNode)
		}
		gp.Nodes.StartTicker(randomNode, message, callback)
	}

}


/*----------------------- Methods for Routing -----------------------*/