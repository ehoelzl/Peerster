package main

import (
	"flag"
	. "github.com/ehoelzl/Peerster/server"
	. "github.com/ehoelzl/Peerster/types"
)

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
	go gp.StartGossipListener()
	go gp.StartClientListener()
	NewServer(address, gp)
}
