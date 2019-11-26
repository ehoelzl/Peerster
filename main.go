package main

import (
	"flag"
	. "github.com/ehoelzl/Peerster/types"
	"log"
)

func StartClientListener(gp *Gossiper) {
	// Listener for client
	for {
		// First read packet
		packetBytes := make([]byte, 1024*16)
		n, _, err := gp.ClientConn.ReadFromUDP(packetBytes)
		if err != nil {
			log.Println("Error reading from UDP from client Port")
		} else {
			go gp.HandleClientMessage(packetBytes[0:n])
		}
	}
}

func StartGossipListener(gp *Gossiper) {
	// Gossip listener
	for {
		// Read packet from other gossipers (always GossipPacket)
		packetBytes := make([]byte, 1024*16)
		n, udpAddr, err := gp.GossipConn.ReadFromUDP(packetBytes)
		if err != nil {
			log.Printf("Error reading from UDP from %v\n", udpAddr.String())
		} else {
			go gp.HandleGossipPacket(udpAddr, packetBytes[0:n])
		}
	}
}

func main() {
	uiPort := flag.String("UIPort", "8080", "port for the UI client (default \"8080\")")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper (default \"127.0.0.1:5000\"")
	name := flag.String("name", "", "name of the gossiper (REQUIRED)")
	peers := flag.String("peers", "", "coma separated list of peers of the form ip:port")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	antiEntropy := flag.Uint("antiEntropy", 10, "Use given timeout in seconds for anti-entropy. If the flag is absent, the default anti-entropy duration is 10 seconds")
	rtimer := flag.Int("rtimer", 0, "Timeout in seconds to send route rumors. 0 (default) means disable sending route rumors.")

	flag.Parse()
	CLIAddress := "127.0.0.1:" + *uiPort
	//GUIListen := "127.0.0.1:8080"
	gp, created := NewGossiper(CLIAddress, *gossipAddr, *name, *peers, *simple, *antiEntropy, *rtimer)
	if created {
		go NewServer(CLIAddress, gp)
		go StartGossipListener(gp)
		StartClientListener(gp)
	}
}
