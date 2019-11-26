package types

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

type Node struct {
	ticker   chan bool
	lastSent *RumorMessage
	udpAddr  *net.UDPAddr
}

// Struct to keep track of known nodes (i.e. IP address)
type Nodes struct {
	nodes map[string]*Node
	sync.RWMutex
}

func InitNodes(addresses string) *Nodes {
	/*Initializes the nodes by parsing the addresses*/
	nodes := make(map[string]*Node)
	if len(addresses) > 0 {
		for _, address := range strings.Split(addresses, ",") {
			peerAddr, err := net.ResolveUDPAddr("udp4", address)
			if err != nil {
				log.Printf("Could not resolve peer address %v\n", address)
				continue
			}
			nodes[peerAddr.String()] = &Node{
				ticker:   nil,
				lastSent: nil,
				udpAddr:  peerAddr,
			}
		}
	}
	return &Nodes{
		nodes: nodes,
	}
}

func (nodes *Nodes) Add(address *net.UDPAddr) {
	/*Adds a new node the the list of nodes*/
	nodes.Lock()
	defer nodes.Unlock()
	if _, ok := nodes.nodes[address.String()]; !ok {
		nodes.nodes[address.String()] = &Node{
			ticker:   nil,
			lastSent: nil,
			udpAddr:  address,
		}
	}
}

func (nodes *Nodes) GetRandom(except map[string]struct{}) (*net.UDPAddr, bool) {
	/*Returns a random node from the list of nodes*/
	var toKeep []*net.UDPAddr
	nodes.RLock()
	for nodeAddr, node := range nodes.nodes {
		if _, noSkip := except[nodeAddr]; except == nil || !noSkip {
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
	/*Prints the list of known nodes*/
	var stringAddresses []string
	nodes.RLock()
	defer nodes.RUnlock()

	for peerAddr, _ := range nodes.nodes {
		stringAddresses = append(stringAddresses, peerAddr)
	}

	fmt.Printf("PEERS %v\n", strings.Join(stringAddresses, ","))
}

func (nodes *Nodes) StartTicker(address *net.UDPAddr, message *RumorMessage, callback func()) {
	/*Starts a ticker for the given node, and the given message. The callback*/
	nodes.Lock()
	defer nodes.Unlock()
	if node, ok := nodes.nodes[address.String()]; ok {
		if node.ticker != nil {
			node.ticker <- true // If other ticker running, kill it
		}
		node.ticker = newTimoutTicker(callback, 10) // Create a new ticker
		node.lastSent = message
	}
}

func (nodes *Nodes) DeleteTicker(address *net.UDPAddr) {
	/*Deletes the ticker for the given address*/
	nodes.Lock()
	defer nodes.Unlock()
	if node, ok := nodes.nodes[address.String()]; ok {
		node.ticker = nil
		node.lastSent = nil
	}
}

func (nodes *Nodes) CheckTimeouts(address *net.UDPAddr) (*RumorMessage, bool) {
	/*Checks if the given node has a timeout running, and returns the last sent message*/
	nodes.Lock()
	defer nodes.Unlock()
	var lastMessage *RumorMessage
	if node, ok := nodes.nodes[address.String()]; ok {
		if node.ticker == nil { // If no ticker running, means no message
			return nil, false
		}
		node.ticker <- true // Kill the ticker
		node.ticker = nil
		lastMessage = node.lastSent // Get the last sent message for CoinFlip
		node.lastSent = nil

		if lastMessage == nil { // Should never go in there
			return nil, false
		}
		return lastMessage, true
	}
	return nil, false
}

func (nodes *Nodes) DistributeBudget(budget uint64) map[*net.UDPAddr]uint64 {
	/*Distributes evenly the Budget amongst known nodes*/
	nodes.RLock()
	defer nodes.RUnlock()
	nodeBudgets := make(map[*net.UDPAddr]uint64)

	for budget > 0 {
		for _, n := range nodes.nodes {
			if _, ok := nodeBudgets[n.udpAddr]; ok {
				nodeBudgets[n.udpAddr] += 1
			} else {
				nodeBudgets[n.udpAddr] = 1
			}
			budget -= 1
			if budget <= 0 {
				break
			}
		}
	}
	return nodeBudgets
}

func (nodes *Nodes) GetAll() []*net.UDPAddr {
	/*Returns all the addresses of nodes*/
	nodes.RLock()
	defer nodes.RUnlock()
	var addresses []*net.UDPAddr
	for _, node := range nodes.nodes {
		addresses = append(addresses, node.udpAddr)
	}
	return addresses
}

func newTimoutTicker(callback func(), seconds time.Duration) chan bool {
	/*Creates a ticker that ticks only once and calls the callback function*/
	ticker := time.NewTicker(seconds * time.Second)
	stop := make(chan bool)
	go func(tick *time.Ticker, stopChan chan bool) {
		for {
			select {
			case <-stopChan:
				tick.Stop()
				return
			case <-tick.C:
				tick.Stop()
				callback()
				return
			}
		}
	}(ticker, stop)
	return stop
}
