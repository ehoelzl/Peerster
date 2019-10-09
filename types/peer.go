package types

import (
	"fmt"
	"strings"
)

type Peer struct {
	Identifier string
	NextID     uint32
	Messages   map[uint32]*RumorMessage
}

func NewPeer(identifier string) *Peer {
	return &Peer{
		Identifier: identifier,
		NextID:     1,
		Messages:   make(map[uint32]*RumorMessage),
	}
}

func (p *Peer) AddMessage(message *RumorMessage) (bool, bool){
	//First check if message ID is == to NextID and corresponds to peer
	if message.Origin != p.Identifier || message.ID < p.NextID{ //Previous message or not same peer
		return false, false
	} else if message.ID > p.NextID { // Message is ahead of current Peer
		return false, true
	} else {
		p.Messages[message.ID] = message
		p.NextID += 1
		return true, false
	}
}

func PrintPeers(peers map[string]*Peer) {
	var stringAddresses []string
	for peerAddr, _ := range peers {
		stringAddresses = append(stringAddresses, peerAddr)
	}
	fmt.Printf("PEERS %v\n", strings.Join(stringAddresses, ","))
}
