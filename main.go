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
	numNodes := flag.Uint64("N", 1, "total number of peers in network (including this node)")
	stubbornTimeout := flag.Uint64("stubbornTimeout", 5, "stubborn time out when sending TLCMessage")
	hopLimit := flag.Uint64("hopLimit", 10, "hop limit for direct messages")
	hw3ex2 := flag.Bool("hw3ex2", false, "flag to use for HW3Ex2")
	hw3ex3 := flag.Bool("hw3ex3", false, "flag to use for HW3Ex3")
	ackAll := flag.Bool("ackAll", false, "ack all incoming unconfirmed TLCMessage")
	hw3ex4 := flag.Bool("hw3ex4", false, "Use blockchain")
	flag.Parse()
	CLIAddress := "0.0.0.0:" + *uiPort
	gp, created := NewGossiper(CLIAddress, *gossipAddr, *name, *peers, *simple, *antiEntropy, *rtimer, *numNodes, *stubbornTimeout, *hw3ex2, uint32(*hopLimit), *hw3ex3, *ackAll, *hw3ex4)
	if created {
		go NewServer(*uiPort, gp)
		go StartGossipListener(gp)
		StartClientListener(gp)
	}
}
