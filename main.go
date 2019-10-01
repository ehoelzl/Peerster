package main

import (
	"flag"
	"fmt"
	. "github.com/ehoelzl/Peerster/types"
)


func main() {
	uiPort := flag.String("UIPort", "8080", "port for the UI client (default \"8080\")")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper (default \"127.0.0.1:5000\"")
	name := flag.String("name", "", "name of the gossiper (REQUIRED)")
	peers := flag.String("peers", "", "coma separated list of peers of the form ip:port")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")

	flag.Parse()
	fmt.Printf("Starting Gossiper node %v:\n -UIPort: %v \n -GossipAddr: %v \n -Peers: %v \n -Simple: %v\n",
		*name, *uiPort, *gossipAddr, *peers, *simple)

	address := "127.0.0.1:" + *uiPort

	gp := NewGossiper(address, *gossipAddr, *name, *peers, *simple)
	go gp.StartGossipListener()
	gp.StartClientListener()

}
