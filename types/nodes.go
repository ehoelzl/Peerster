package types

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
)

type Node struct {
	LastStatus chan *StatusPacket
	IsOpen     bool
	udpAddr    *net.UDPAddr
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
					LastStatus: make(chan *StatusPacket),
					IsOpen:     false,
					udpAddr:    peerAddr,
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
		LastStatus: make(chan *StatusPacket),
		IsOpen:     false,
		udpAddr:    address,
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

func (nodes *Nodes) CloseChannel(address *net.UDPAddr) {
	nodes.Lock.Lock()
	defer nodes.Lock.Unlock()
	if elem, ok := nodes.Addresses[address.String()]; ok {
		if elem.IsOpen {
			elem.IsOpen = false
			//close(elem.LastStatus)
			//elem.LastStatus = nil
		}
	}
}

func (nodes *Nodes) OpenChannel(address *net.UDPAddr) {
	nodes.Lock.Lock()
	defer nodes.Lock.Unlock()
	if elem, ok := nodes.Addresses[address.String()]; ok {
		if !elem.IsOpen {
			elem.IsOpen = true
			//elem.LastStatus = make(chan *StatusPacket)
		}
	}
}

func (nodes *Nodes) IsOpen(address *net.UDPAddr) bool{
	nodes.Lock.RLock()
	defer nodes.Lock.RUnlock()
	return nodes.Addresses[address.String()].IsOpen
}