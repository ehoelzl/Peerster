package types

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
)

type Node struct {
	Channels map[*RumorMessage]chan bool
	LastSent *RumorMessage
	udpAddr  *net.UDPAddr
}
type Nodes struct {
	Addresses map[string]*Node
	Lock      sync.RWMutex
}

func NewNodes(addresses string) *Nodes {
	nodes := make(map[string]*Node)
	if len(addresses) > 0 {
		for _, address := range strings.Split(addresses, ",") {
			peerAddr, err := net.ResolveUDPAddr("udp4", address)
			if err == nil {
				nodes[peerAddr.String()] = &Node{
					Channels: make(map[*RumorMessage]chan bool),
					LastSent: nil,
					udpAddr:  peerAddr,
				}
			}
		}
	}
	return &Nodes{
		Addresses: nodes,
		Lock:      sync.RWMutex{},
	}
}

func (nodes *Nodes) AddNode(address *net.UDPAddr) {
	// Checks if the given `*net.UDPAddr` is in `gp.Nodes`, if not, it adds it.
	nodes.Lock.Lock()
	defer nodes.Lock.Unlock()

	for nodeAddr, _ := range nodes.Addresses {
		if nodeAddr == address.String() {
			return
		}
	}
	nodes.Addresses[address.String()] = &Node{
		Channels: make(map[*RumorMessage]chan bool),
		LastSent: nil,
		udpAddr:  address,
	}
}

func (nodes *Nodes) RandomNode(except map[string]struct{}) *net.UDPAddr {
	// Returns a random address from the list of Addresses
	var toKeep []*net.UDPAddr
	nodes.Lock.RLock()
	for nodeAddr, node := range nodes.Addresses {
		_, noSkip := except[nodeAddr] // If address in `except`
		if except == nil || !noSkip {
			toKeep = append(toKeep, node.udpAddr) // If the address of the peer is different than `except` we add it to the list
		}
	}
	nodes.Lock.RUnlock()

	if len(toKeep) == 0 {
		return nil
	}
	randInt := rand.Intn(len(toKeep)) // Choose random number
	return toKeep[randInt]
}

func (nodes *Nodes) Print() {
	var stringAddresses []string
	nodes.Lock.RLock()
	defer nodes.Lock.RUnlock()

	for peerAddr, _ := range nodes.Addresses {
		stringAddresses = append(stringAddresses, peerAddr)
	}

	fmt.Printf("PEERS %v\n", strings.Join(stringAddresses, ","))
}

func (nodes *Nodes) RegisterChannel(address *net.UDPAddr, rumor *RumorMessage, channel chan bool) {
	// Registers the given channel for the given message
	nodes.Lock.Lock()
	defer nodes.Lock.Unlock()
	if node, ok := nodes.Addresses[address.String()]; ok {
		if ch, ok := node.Channels[rumor]; ok { // If we already have a channel for this rumor, node, close it
			close(ch)
		}
		nodes.Addresses[address.String()].Channels[rumor] = channel
		nodes.Addresses[address.String()].LastSent = rumor
	}
}

func (nodes *Nodes) DeleteChannel(address *net.UDPAddr, rumor *RumorMessage) {
	nodes.Lock.Lock()
	defer nodes.Lock.Unlock()
	if node, ok := nodes.Addresses[address.String()]; ok {
		if ch, ok := node.Channels[rumor]; ok {
			close(ch)
			delete(nodes.Addresses[address.String()].Channels, rumor)
		}
	}
}

func (nodes *Nodes) CheckTimeouts(address *net.UDPAddr, status *StatusPacket) *RumorMessage {
	// Checks for timers on sent messages, stops them and returns the last acked Rumor, or nil if no rumors were waiting for ack
	nodes.Lock.Lock()
	defer nodes.Lock.Unlock()

	statusMap := status.ToMap()
	var acked []*RumorMessage
	var lastMessage *RumorMessage

	if node, ok := nodes.Addresses[address.String()]; ok { // Check if channels are open for tis node
		for rumor, ack := range node.Channels { // iterate over all channels for this node
			if elem, ok := statusMap[rumor.Origin]; ok { // Means the rumor's origin is in statusMap
				if elem > rumor.ID { // Means this rumor is acked
					ack <- true
					acked = append(acked, rumor)
				}
			}
		}

		for _, r := range acked {
			close(node.Channels[r])
			delete(node.Channels, r)
		}

		if len(acked) > 0 {
			lastMessage = node.LastSent
		}

	}
	return lastMessage
}
