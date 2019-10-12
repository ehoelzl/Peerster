package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	. "github.com/ehoelzl/Peerster/server"
	. "github.com/ehoelzl/Peerster/types"
	"github.com/ehoelzl/Peerster/utils"
)

func StartClientListener(gp *Gossiper) {
	// Listener for client
	packetBytes := make([]byte, 1024) // TODO maybe increase size
	packet := &Message{}
	for {
		// First read packet
		n, _, err := gp.ClientConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from client Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from client Port for %v\n", gp.Name))
		fmt.Printf("CLIENT MESSAGE %v\n", packet.Text)

		go gp.HandleClientMessage(packet)
	}
}
func StartGossipListener(gp *Gossiper) {
	// Gossip listener
	packetBytes := make([]byte, 1024)
	packet := &GossipPacket{}
	for {
		// Read packet from other gossipers (always RumorMessage)
		n, udpAddr, err := gp.GossipConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from gossip Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from gossip Port for %v\n", gp.Name))

		go gp.HandleGossipPacket(udpAddr, packet)
	}
}

func main() {
	uiPort := flag.String("UIPort", "8080", "port for the UI client (default \"8080\")")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper (default \"127.0.0.1:5000\"")
	name := flag.String("name", "", "name of the gossiper (REQUIRED)")
	peers := flag.String("peers", "", "coma separated list of peers of the form ip:port")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	antiEntropy := flag.Uint("antiEntropy", 10, "AntiEntropy value (default 10 seconds)")

	flag.Parse()
	address := "127.0.0.1:" + *uiPort
	gp := NewGossiper(address, *gossipAddr, *name, *peers, *simple, *antiEntropy)
	go StartGossipListener(gp)
	go StartClientListener(gp)
	NewServer(address, gp)
}
