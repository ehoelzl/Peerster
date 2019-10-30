package types

import (
	"fmt"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

type Node struct {
	Ticker   *time.Ticker
	LastSent *RumorMessage
	udpAddr  *net.UDPAddr
}
type Nodes struct {
	nodes map[string]*Node
	sync.RWMutex
}

func NewNodes(addresses string) *Nodes {
	// Parses the string addresses passed in CLI and stores them in a Map ofr Node
	nodes := make(map[string]*Node)
	if len(addresses) > 0 {
		for _, address := range strings.Split(addresses, ",") {
			peerAddr, err := net.ResolveUDPAddr("udp4", address)
			if err != nil {
				log.Printf("Could not resolve peer address %v\n", address)
				continue
			}
			nodes[peerAddr.String()] = &Node{
				Ticker:   nil,
				LastSent: nil,
				udpAddr:  peerAddr,
			}
		}
	}
	return &Nodes{
		nodes: nodes,
	}
}

func (nodes *Nodes) Add(address *net.UDPAddr) {
	// Checks if the given `*net.UDPAddr` is in `gp.Nodes`, if not, it adds it.
	nodes.Lock()
	defer nodes.Unlock()
	if _, ok := nodes.nodes[address.String()]; !ok {
		nodes.nodes[address.String()] = &Node{
			Ticker:   nil,
			LastSent: nil,
			udpAddr:  address,
		}
	}
}

func (nodes *Nodes) GetRandom(except map[string]struct{}) (*net.UDPAddr, bool) {
	// Returns a random address from the list of nodes
	var toKeep []*net.UDPAddr
	nodes.RLock()
	for nodeAddr, node := range nodes.nodes {
		_, noSkip := except[nodeAddr] // If address in `except`
		if except == nil || !noSkip {
			toKeep = append(toKeep, node.udpAddr) // If the address of the peer is different than `except` we add it to the list
		}
	}
	nodes.RUnlock()

	if len(toKeep) == 0 {
		return nil, false
	}
	randInt := rand.Intn(len(toKeep)) // Choose random number
	return toKeep[randInt], true
}

func (nodes *Nodes) Print() {
	var stringAddresses []string
	nodes.RLock()
	defer nodes.RUnlock()

	for peerAddr, _ := range nodes.nodes {
		stringAddresses = append(stringAddresses, peerAddr)
	}

	fmt.Printf("PEERS %v\n", strings.Join(stringAddresses, ","))
}

func (nodes *Nodes) StartTicker(address *net.UDPAddr, message *RumorMessage, callback func()) {
	// Registers the given channel for the given message
	nodes.Lock()
	defer nodes.Unlock()
	if node, ok := nodes.nodes[address.String()]; ok {
		if node.Ticker != nil {
			node.Ticker.Stop() // Stop the running ticker
		}
		node.Ticker = utils.NewTimoutTicker(callback, 10)
		node.LastSent = message
	}
}

func (nodes *Nodes) DeleteTicker(address *net.UDPAddr) {
	nodes.Lock()
	defer nodes.Unlock()
	if node, ok := nodes.nodes[address.String()]; ok {
		node.Ticker.Stop()
		node.Ticker = nil
		node.LastSent = nil
	}
}

func (nodes *Nodes) CheckTimeouts(address *net.UDPAddr) (*RumorMessage, bool) {
	nodes.Lock()
	defer nodes.Unlock()
	var lastMessage *RumorMessage
	if node, ok := nodes.nodes[address.String()]; ok {
		if node.Ticker == nil {
			return nil, false
		}
		node.Ticker.Stop()
		node.Ticker = nil
		lastMessage = node.LastSent
		node.LastSent = nil
		if lastMessage == nil { // Should never go in there
			log.Println("Got non nil ticker with nil message")
			return nil, false
		}
		return lastMessage, true

	}
	return nil, false
}

func (nodes *Nodes) GetAll() map[string]*Node {
	// Function only used by server
	nodes.RLock()
	defer nodes.RUnlock()
	return nodes.nodes
}