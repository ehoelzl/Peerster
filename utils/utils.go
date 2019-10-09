package utils

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

type PeerTicker struct {
	AckReceived chan bool
	ID          uint32
}

func CheckError(err error, msg string) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}

func ParseAddresses(addresses string) []*net.UDPAddr {
	var peerAddresses []*net.UDPAddr
	if len(addresses) > 0 {
		for _, peer := range strings.Split(addresses, ",") {
			peerAddr, err := net.ResolveUDPAddr("udp4", peer)
			if err != nil {
				fmt.Printf("Could not resolve peer address at %v\n", peer)
			} else {
				peerAddresses = append(peerAddresses, peerAddr)
			}
		}
	}
	return peerAddresses
}

func PrintAddresses(addresses []*net.UDPAddr) {
	var stringAddresses []string
	for _, peerAddr := range addresses {
		stringAddresses = append(stringAddresses, peerAddr.String())
	}
	fmt.Printf("PEERS %v\n", strings.Join(stringAddresses, ","))
}

func RandomNode(nodes []*net.UDPAddr, except map[string]struct{}) *net.UDPAddr {
	// Returns a random Peer from the list of peers
	var toKeep []*net.UDPAddr
	for _, address := range nodes {
		_, noSkip := except[address.String()] // If address in `except`
		if except == nil || !noSkip {
			toKeep = append(toKeep, address) // If the address of the peer is different than `except` we add it to the list
		}
	}
	if len(toKeep) == 0 {
		//panic(fmt.Sprint("ERROR: Trying to fetch random peer from empty array"))
		return nil
	}
	randInt := rand.Intn(len(toKeep)) // Choose random number
	return toKeep[randInt]
}

func NewPeerTimer(messageId uint32, callback func(), seconds time.Duration) *PeerTicker {
	receivedAck := make(chan bool)
	go func() {
		ticker := time.NewTicker(seconds * time.Second)
		for {
			select {
			case <-receivedAck:
				ticker.Stop()
			case <-ticker.C:
				fmt.Print("TIMEMOUT\n")
				ticker.Stop()
				callback()
			}
		}
	}()
	return &PeerTicker{
		AckReceived: receivedAck,
		ID:          messageId,
	}
}
