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
}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool, antiEntropy uint) *Gossiper {
	// Creates new gossiper with the given parameters
	clientAddr, err := net.ResolveUDPAddr("udp4", uiAddress)
	clientConn, err := net.ListenUDP("udp4", clientAddr) // Connection to client
	utils.CheckError(err, fmt.Sprintf("Error when opening client UDP channel for %v\n", name))

	gossipAddr, err := net.ResolveUDPAddr("udp4", gossipAddress)
	gossipConn, err := net.ListenUDP("udp4", gossipAddr)
	utils.CheckError(err, fmt.Sprintf("Error when opening gossip UDP channel for %v\n", name))
	log.Printf("Starting gossiper %v\n UIAddress: %v\n GossipAddress %v\n Peers %v\n\n", name, clientAddr, gossipAddr, initialPeers)

	peers := NewPeers()
	peers.AddPeer(name)
	gossiper := &Gossiper{
		ClientAddress: clientAddr,
		ClientConn:    clientConn,
		GossipAddress: gossipAddr,
		GossipConn:    gossipConn,
		Name:          name,
		IsSimple:      simple,
		Nodes:         NewNodes(initialPeers),
		Peers:         peers,
	}

	go utils.NewTicker(func() {
		randomNode := gossiper.Nodes.RandomNode(nil)
		if randomNode != nil {
			go gossiper.SendStatusMessage(randomNode)
		}
	}, time.Duration(antiEntropy))

	return gossiper
}

func (gp *Gossiper) HandleClientMessage(message *Message) {
	if message == nil {
		log.Println("Got nil message from client")
		return
	}
	fmt.Printf("CLIENT MESSAGE %v\n", message.Text)

	if gp.IsSimple { // Simple case
		simpleMessage := &SimpleMessage{
			OriginalName:  gp.Name,
			RelayPeerAddr: gp.GossipAddress.String(),
			Contents:      message.Text,
		}
		go gp.SimpleBroadcast(simpleMessage, nil)
	} else {
		message := &RumorMessage{
			Origin: gp.Name,
			ID:     gp.Peers.NextId(gp.Name),
			Text:   message.Text,
		}
		messageAdded := gp.Peers.AddRumorMessage(message) // Usually, message is always added
		if messageAdded {                                 //Rumor Only if message was added (i.e. never seen before and is coherent with NextID)
			randomNode := gp.Nodes.RandomNode(nil) // Pick random node
			go gp.StartRumormongering(randomNode, message, nil, false, true)
		}
	}
}

func (gp *Gossiper) HandleGossipPacket(from *net.UDPAddr, packet *GossipPacket) {
	if packet == nil {
		log.Println("Got nil packet from gossiper")
		return
	}

	gp.Nodes.AddNode(from) // Add node to list if not in
	if gp.IsSimple {
		if packet.Simple == nil {
			log.Printf("Empty simple message from %v\n", from.String())
			return
		}
		fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v \n", packet.Simple.OriginalName, packet.Simple.RelayPeerAddr, packet.Simple.Contents)
		gp.Nodes.Print()

		packet.Simple.RelayPeerAddr = gp.GossipAddress.String()
		go gp.SimpleBroadcast(packet.Simple, from)
	} else {
		if rumor := packet.Rumor; rumor != nil {
			go gp.HandleRumorMessage(from, rumor)

		} else if status := packet.Status; status != nil {
			go gp.HandleStatusPacket(from, status)

		} else {
			log.Printf("Empty packet from %v\n", from.String())
		}
	}
}

func (gp *Gossiper) HandleRumorMessage(from *net.UDPAddr, rumor *RumorMessage) {
	// Assumes rumor is not nil
	fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n", rumor.Origin, from.String(), rumor.ID, rumor.Text)
	gp.Nodes.Print()

	messageAdded := gp.Peers.AddRumorMessage(rumor) // Add message to list
	go gp.SendStatusMessage(from)                 // Send back ack

	if messageAdded { // If message was not seen before, continue rumor mongering to other nodes
		except := map[string]struct{}{from.String(): struct{}{}} // Monger with other nodes except this one
		randomNode := gp.Nodes.RandomNode(except) // Pick random node
		go gp.StartRumormongering(randomNode, rumor, except, false, true)
	}
}

func (gp *Gossiper) HandleStatusPacket(from *net.UDPAddr, status *StatusPacket) {
	status.PrintStatusMessage(from)
	gp.Nodes.Print()
	gp.Nodes.Addresses[from.String()].LastStatus <- status
/*	if gp.Nodes.IsOpen(from) { // Here means that this is not an ack
		fmt.Println("Ack")
		gp.Nodes.Addresses[from.String()].LastStatus <- status
	} else {
		fmt.Println("Not Ack")
		go gp.CheckStatus(from, status, false, nil)
	}*/
}

func (gp *Gossiper) CheckStatus(from *net.UDPAddr, status *StatusPacket, isAck bool, message *RumorMessage) {
	missing := gp.Peers.GetTheirMissingMessage(status)
	isMissing := gp.Peers.IsMissingMessage(status)
	inSync := missing == nil && !isMissing
	if inSync {
		fmt.Printf("IN SYNC WITH %v\n", from.String())
		//gp.Nodes.CloseChannel(from)
		if isAck {
			/*if flip := utils.CoinFlip(); flip && message != nil{
				except := map[string]struct{}{from.String(): struct{}{}} // Monger with other nodes except this one
				randomNode := gp.Nodes.RandomNode(except)
				//fmt.Println(randomNode, message.Text)
				go gp.StartRumormongering(randomNode, message, except, true, false)
			}*/
		}
	} else {
		if missing != nil {
			go gp.StartRumormongering(from, missing, nil, false, false) // Start rumorMongering all over again
		} else if isMissing {
			gp.SendStatusMessage(from) // I have missing messages
		}
	}
}

/*-------------------- Methods used for transferring messages and broadcasting ------------------------------*/

func (gp *Gossiper) SendPacket(simple *SimpleMessage, rumor *RumorMessage, status *StatusPacket, to *net.UDPAddr) {
	gossipPacket, err := protobuf.Encode(&GossipPacket{Simple: simple, Rumor: rumor, Status: status})
	if err == nil {
		_, err = gp.GossipConn.WriteToUDP(gossipPacket, to)
		if err != nil {
			log.Printf("Error sending gossipPacket from node %v to node %v\n", gp.GossipAddress.String(), to.String())
		}
	} else {
		log.Printf("Error encoding gossipPacket for %v\n", to.String())
	}
}

func (gp *Gossiper) SendStatusMessage(to *net.UDPAddr) {
	// Creates the vector clock for the given peer
	statusMessages := gp.Peers.GetStatusMessage()
	statusPacket := &StatusPacket{Want: statusMessages}
	gp.SendPacket(nil, nil, statusPacket, to)
}

func (gp *Gossiper) SimpleBroadcast(packet *SimpleMessage, except *net.UDPAddr) {
	// Double functionality: Broadcasts to all peers if except == nil, or to all except the given one
	gp.Nodes.Lock.RLock()
	defer gp.Nodes.Lock.RUnlock()

	for nodeAddr, node := range gp.Nodes.Addresses {
		if nodeAddr != except.String() {
			gp.SendPacket(packet, nil, nil, node.udpAddr)
		}
	}
}

func (gp *Gossiper) StartRumormongering(node *net.UDPAddr, message *RumorMessage, except map[string]struct{}, coinFlip bool, timeout bool) {

	if node != nil {
		if coinFlip {
			fmt.Printf("FLIPPED COIN sending rumor to %v\n", node.String())
		} else {
			fmt.Printf("MONGERING with %v\n", node.String())
		}

		if except == nil {
			except = make(map[string]struct{})
		}
		except[node.String()] = struct{}{} // Add node we send to, to the except list

		//gp.Nodes.OpenChannel(node)
		go gp.SendPacket(nil, message, nil, node) // Send rumor

		ticker := time.NewTicker(10 * time.Second) // Start ticker

		go func() {
			//defer gp.Nodes.CloseChannel(node) // CLose the channel upon completion of rumor mongering
			for {
				select {
				case status := <-gp.Nodes.Addresses[node.String()].LastStatus: // Wait for status
					// Check if status acknowledges sent message, if not ignore
					if status.IsAckFor(message){
						go gp.CheckStatus(node, status, true, message)
						ticker.Stop()
						return
					}

				case <-ticker.C:
					if timeout{
						fmt.Printf("Timeout for message %v sent to %v\n", message, node)
						randomNode := gp.Nodes.RandomNode(except) // Pick random node
						go gp.StartRumormongering(randomNode, message, except, false, true) // If we time out, then start again
					}
					ticker.Stop()
					return
				}
			}
		}()
	}
}
