package types

import (
	"fmt"
	"net"
	"strings"
)

type Message struct {
	Text        string
	Destination string
	File        string
	Request     *[]byte
}

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

type GossipPacket struct {
	Simple  *SimpleMessage
	Rumor   *RumorMessage
	Status  *StatusPacket
	Private *PrivateMessage
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

func (sp *StatusPacket) PrintStatusMessage(sender *net.UDPAddr) {
	peerStatus := sp.Want
	var statusString []string
	for _, p := range peerStatus {
		statusString = append(statusString, fmt.Sprintf("peer %v nextID %v", p.Identifier, p.NextID))
	}
	fmt.Printf("STATUS from %v %v\n", sender.String(), strings.Join(statusString, " "))
}

func (sp *StatusPacket) ToMap() map[string]uint32 {
	statusMap := make(map[string]uint32)
	for _, ps := range sp.Want {
		statusMap[ps.Identifier] = ps.NextID
	}
	return statusMap
}
