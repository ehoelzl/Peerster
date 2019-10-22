package main

import (
	"flag"
	"github.com/dedis/protobuf"
	. "github.com/ehoelzl/Peerster/types"
	"log"
)

func StartClientListener(gp *Gossiper) {
	// Listener for client
	packetBytes := make([]byte, 1024) // TODO maybe increase size
	packet := &Message{}
	for {
		// First read packet
		n, _, err := gp.ClientConn.ReadFromUDP(packetBytes)
		if err == nil {
			err = protobuf.Decode(packetBytes[0:n], packet)
			if err == nil && packet != nil {
				go gp.HandleClientMessage(packet)
			} else if packet == nil {
				log.Println("Got nil packet from client")
			} else {
				log.Println("Error decoding packet from client")
			}
		} else {
			log.Println("Error reading from UDP from client Port")
		}
	}
}

func StartGossipListener(gp *Gossiper) {
	// Gossip listener
	packetBytes := make([]byte, 1024)
	packet := &GossipPacket{}
	for {
		// Read packet from other gossipers (always RumorMessage)
		n, udpAddr, err := gp.GossipConn.ReadFromUDP(packetBytes)
		if err == nil {
			err = protobuf.Decode(packetBytes[0:n], packet)
			if err == nil && packet != nil {
				go gp.HandleGossipPacket(udpAddr, packet)
			} else if packet == nil {
				log.Printf("Got nil packet from %v\n", udpAddr.String())
			} else {
				log.Printf("Error decoding packet from %v\n", udpAddr.String())
			}
		} else {
			log.Printf("Error reading from UDP from %v\n", udpAddr.String())
		}
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
	CLIAddress := "127.0.0.1:" + *uiPort
	GUIListen := "127.0.0.1:8080"
	gp := NewGossiper(CLIAddress, *gossipAddr, *name, *peers, *simple, *antiEntropy)
	go NewServer(GUIListen, gp)
	go StartGossipListener(gp)
	StartClientListener(gp)
}
