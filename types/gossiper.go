package types

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/ehoelzl/Peerster/utils"
	"net"
	"strings"
)

type Gossiper struct {
	clientAddress *net.UDPAddr
	clientConn    *net.UDPConn
	gossipAddress *net.UDPAddr
	gossipConn    *net.UDPConn
	Name          string
	peers         []string
	simple        bool
}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool) *Gossiper {
	udpAddr, err := net.ResolveUDPAddr("udp4", uiAddress)
	utils.CheckError(err, fmt.Sprintf("Error when resolving client address %v for %v \n", uiAddress, name))

	udpConn, err := net.ListenUDP("udp4", udpAddr) // Connection to client
	utils.CheckError(err, fmt.Sprintf("Error when opening client UDP channel for %v\n", name))

	gossipAddr, err := net.ResolveUDPAddr("udp4", gossipAddress)
	utils.CheckError(err, fmt.Sprintf("Error when resolving gossip address %v for %v \n", gossipAddress, name))

	gossipConn, err := net.ListenUDP("udp4", gossipAddr) // Connection to client
	utils.CheckError(err, fmt.Sprintf("Error when opening gossip UDP channel for %v\n", name))
	return &Gossiper{
		clientAddress: udpAddr,
		clientConn:    udpConn,
		gossipAddress: gossipAddr,
		gossipConn:    gossipConn,
		Name:          name,
		peers:         strings.Split(initialPeers, ","),
		simple:        simple,
	}
}

func (gp *Gossiper) StartClientListener() {
	fmt.Printf("Gossip %v listening on UIPort %v\n\n", gp.Name, gp.clientAddress)
	packetBytes := make([]byte, 1024)
	packet := &ClientMessage{}
	for {
		n, _, err := gp.clientConn.ReadFromUDP(packetBytes)
		if err != nil {
			panic(err)
		}
		err = protobuf.Decode(packetBytes[0:n], packet)
		if err != nil {
			panic(err)
		}
		fmt.Println("CLIENT MESSAGE", packet.Contents)
	}
}

func (gp *Gossiper) StartGossipListener() {
	fmt.Printf("Gossip %v listening on GossipPort %v\n\n", gp.Name, gp.gossipAddress)
	packetBytes := make([]byte, 1024)
	packet := &GossipPacket{}
	for {
		n, addr, err := gp.gossipConn.ReadFromUDP(packetBytes)
		if err != nil {
			panic(err)
		}
		err = protobuf.Decode(packetBytes[0:n], packet)
		if err != nil {
			panic(err)
		}
		fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v \n", packet.Simple.OriginalName, addr, packet.Simple.Contents)
	}
}
