package types

import (
	"fmt"
	"net"
	"strings"
)

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

type RumorMessage struct { // No need to add RelayPeerAddr as it is deduced from sending node
	Origin string
	ID     uint32
	Text   string
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

func (message *StatusPacket) PrintStatusMessage(sender *net.UDPAddr) {
	peerStatus := message.Want
	var statusString []string
	for _, p := range peerStatus {
		statusString = append(statusString, fmt.Sprintf("peer %v nextID %v", p.Identifier, p.NextID))
	}
	fmt.Printf("STATUS from %v %v\n", sender.String(), strings.Join(statusString, " "))
}
