package types

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
)

type Nodes struct {
	Addresses []*net.UDPAddr
	Lock      sync.RWMutex
}

func NewNodes(addresses string) *Nodes {
	var peerAddresses []*net.UDPAddr
	if len(addresses) > 0 {
		for _, peer := range strings.Split(addresses, ",") {
			peerAddr, err := net.ResolveUDPAddr("udp4", peer)
			if err != nil {
				log.Printf("Could not resolve peer address at %v\n", peer)
			} else {
				peerAddresses = append(peerAddresses, peerAddr)
			}
		}
	}
	return &Nodes{
		Addresses: peerAddresses,
		Lock:      sync.RWMutex{},
	}
}

func (nodes *Nodes) AddNode(address *net.UDPAddr) {
	// Checks if the given `*net.UDPAddr` is in `gp.Nodes`, if not, it adds it.
	nodes.Lock.Lock()
	defer nodes.Lock.Unlock()

	for _, nodeAddr := range nodes.Addresses {
		if nodeAddr.String() == address.String() {
			return
		}
	}
	nodes.Addresses = append(nodes.Addresses, address)
}

func (nodes *Nodes) RandomNode(except map[string]struct{}) *net.UDPAddr {
	// Returns a random address from the list of Addresses
	var toKeep []*net.UDPAddr
	nodes.Lock.RLock()
	for _, address := range nodes.Addresses {
		_, noSkip := except[address.String()] // If address in `except`
		if except == nil || !noSkip {
			toKeep = append(toKeep, address) // If the address of the peer is different than `except` we add it to the list
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

	for _, peerAddr := range nodes.Addresses {
		stringAddresses = append(stringAddresses, peerAddr.String())
	}

	fmt.Printf("PEERS %v\n", strings.Join(stringAddresses, ","))
}
