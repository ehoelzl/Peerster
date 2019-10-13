package types

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"time"

	//"log"
	"net"
	"sync"
)

type Gossiper struct {
	ClientAddress *net.UDPAddr
	ClientConn    *net.UDPConn
	GossipAddress *net.UDPAddr
	GossipConn    *net.UDPConn
	IsSimple      bool
	Name          string
	Nodes         *Nodes
	Peers         *GossipPeers     // From name to peer
	Tickers       *Tickers
	Buffer        map[string]*RumorMessage // Keeps track of the last rumor sent to this node
	BufferLock    sync.RWMutex
}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool, antiEntropy uint) *Gossiper {
	// Creates new gossiper with the given parameters
	clientAddr, err := net.ResolveUDPAddr("udp4", uiAddress)
	clientConn, err := net.ListenUDP("udp4", clientAddr) // Connection to client
	utils.CheckError(err, fmt.Sprintf("Error when opening client UDP channel for %v\n", name))

	gossipAddr, err := net.ResolveUDPAddr("udp4", gossipAddress)
	gossipConn, err := net.ListenUDP("udp4", gossipAddr)
	utils.CheckError(err, fmt.Sprintf("Error when opening gossip UDP channel for %v\n", name))
	fmt.Printf("Starting gossiper %v\n UIAddress: %v\n GossipAddress %v\n Peers %v\n\n", name, clientAddr, gossipAddr, initialPeers)

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
		Tickers:       NewTickers(),
		Buffer:        make(map[string]*RumorMessage),
		BufferLock:    sync.RWMutex{},
	}

	go utils.NewTicker(func() {
		randomNode := gossiper.Nodes.RandomNode(nil)
		if randomNode != nil {
			gossiper.SendStatusMessage(randomNode)
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

	if gp.IsSimple {
		simpleMessage := &SimpleMessage{
			OriginalName:  gp.Name,
			RelayPeerAddr: gp.GossipAddress.String(),
			Contents:      message.Text,
		} // TODO : add message
		go gp.SimpleBroadcast(simpleMessage, nil)
	} else {
		message := &RumorMessage{
			Origin: gp.Name,
			ID:     gp.Peers.NextId(gp.Name),
			Text:   message.Text,
		}
		messageAdded := gp.Peers.AddRumorMessage(message) // Usually, message is always added
		if messageAdded {                           //Rumor Only if message was added (i.e. never seen before and is coherent with NextID)
			go gp.StartRumormongering(message, nil, false)
		}
	}
}

func (gp *Gossiper) HandleRumorMessage(from *net.UDPAddr, rumor *RumorMessage) {
	// Assumes rumor is not nil
	fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n", rumor.Origin, from.String(), rumor.ID, rumor.Text)
	gp.Nodes.Print()

	messageAdded := gp.Peers.AddRumorMessage(rumor) // Add message to list
	go gp.SendStatusMessage(from)             // Send back ack
	if messageAdded {                         // If message was not seen before, continue rumor mongering to other nodes
		except := map[string]struct{}{from.String(): struct{}{}} // Monger with other nodes except this one
		go gp.StartRumormongering(rumor, except, false)
	}
	fmt.Println()
}

func (gp *Gossiper) HandleStatusPacket(from *net.UDPAddr, status *StatusPacket) {
	// Assumes status is not nil
	status.PrintStatusMessage(from)
	gp.Nodes.Print()

	isAck := gp.Tickers.DeleteTicker(from)

	missing := gp.Peers.GetTheirMissingMessage(status)
	isMissing := gp.Peers.IsMissingMessage(status)
	inSync := missing == nil && !isMissing

	if inSync {
		fmt.Printf("IN SYNC WITH %v\n", from.String())
		if isAck{
			flip := utils.CoinFlip()
			fmt.Printf("Is Ack: flip ? %v\n", flip)
			gp.BufferLock.RLock()
			lastMessage, ok := gp.Buffer[from.String()]
			gp.BufferLock.RUnlock()
			if ok && flip {
				except := map[string]struct{}{from.String(): struct{}{}}
				go gp.StartRumormongering(lastMessage, except, true)
			}
		}


	} else {
		if missing != nil {
			go gp.SendRumorMessage(missing, from, nil)
		} else if isMissing {
			go gp.SendStatusMessage(from)
		}
	}
	fmt.Println()
}

func (gp *Gossiper) HandleGossipPacket(from *net.UDPAddr, packet *GossipPacket) {
	if packet == nil {
		log.Println("Got nil packet from gossiper")
		return
	}

	gp.Nodes.AddNode(from)
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
			fmt.Printf("Empty packet from %v\n", from.String())
		}
	}
}


/*-------------------- Methods used for transferring messages and broadcasting ------------------------------*/

func (gp *Gossiper) SendPacket(simple *SimpleMessage, rumor *RumorMessage, status *StatusPacket, to *net.UDPAddr) {
	gossipPacket, err := protobuf.Encode(&GossipPacket{Simple: simple, Rumor: rumor, Status: status})
	if err == nil {
		_, err = gp.GossipConn.WriteToUDP(gossipPacket, to)
		if err != nil {
			fmt.Printf("Error sending gossipPacket from node %v to node %v\n", gp.GossipAddress.String(), to.String())
		}
	} else {
		fmt.Printf("Error encoding gossipPacket for %v\n", to.String())
	}
}

func (gp *Gossiper) SendStatusMessage(to *net.UDPAddr) {
	// Creates the vector clock for the given peer
	statusMessages := gp.Peers.GetStatusMessage()
	statusPacket := &StatusPacket{Want: statusMessages}
	go gp.SendPacket(nil, nil, statusPacket, to)
	//log.Printf("Sent status to %v\n\n", to.String())
}

func (gp *Gossiper) SimpleBroadcast(packet *SimpleMessage, except *net.UDPAddr) {
	// Double functionality: Broadcasts to all peers if except == nil, or to all except the given one
	gp.Nodes.Lock.RLock()
	defer gp.Nodes.Lock.RUnlock()

	for _, nodeAddr := range gp.Nodes.Addresses {
		if nodeAddr.String() != except.String() {
			go gp.SendPacket(packet, nil, nil, nodeAddr)
		}
	}
}

func (gp *Gossiper) SendRumorMessage(message *RumorMessage, to *net.UDPAddr, callback func()) {
	// Sends the message to the given node, and creates a timer
	go gp.SendPacket(nil, message, nil, to)

	// Then start ticker for this node/origin/messageId
	gp.Tickers.AddTicker(to, callback, 10)

	gp.BufferLock.Lock()
	gp.Buffer[to.String()] = message // Add message to buffer
	gp.BufferLock.Unlock()
}

func (gp *Gossiper) StartRumormongering(message *RumorMessage, except map[string]struct{}, coinFlip bool) {
	// Picks random node and sends RumorMessage
	randomNode := gp.Nodes.RandomNode(except)
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
			go gp.StartRumormongering(message, except, false) // Rumormonger with other nodes
			gp.Tickers.DeleteTicker(randomNode)
		}
		gp.SendRumorMessage(message, randomNode, callback)
		fmt.Println()
	}
}


