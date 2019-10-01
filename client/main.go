package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	. "github.com/ehoelzl/Peerster/types"
	"net"
)

func main() {
	uiPort := flag.String("UIPort", "8080", "port for the UI client (default \"8080\")")
	msg := flag.String("msg", "", "message to be sent (REQUIRED)")
	flag.Parse()
	fmt.Printf("Message %v to be sent to %v\n", *msg, *uiPort)

	message := ClientMessage{Contents: *msg}
	packetBytes, err := protobuf.Encode(&message)
	if err != nil {
		panic(err)
	}
	address := "127.0.0.1:" + *uiPort
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	udpConn, _ := net.DialUDP("udp4", nil, udpAddr)
	_, err = udpConn.Write(packetBytes)
	if err != nil {
		panic(err)
	}
}
