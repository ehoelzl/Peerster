package main

import (
	"flag"
	"github.com/dedis/protobuf"
	. "github.com/ehoelzl/Peerster/types"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"net"
)

func main() {
	uiPort := flag.String("UIPort", "8080", "port for the UI client (default \"8080\")")
	msg := flag.String("msg", "", "message to be sent (REQUIRED)")
	flag.Parse()
	log.Printf("Message %v to be sent to %v\n", *msg, *uiPort)

	message := Message{Text: *msg}
	packetBytes, err := protobuf.Encode(&message)
	utils.CheckError(err, "Error encoding message")

	address := "127.0.0.1:" + *uiPort
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	udpConn, _ := net.DialUDP("udp4", nil, udpAddr)
	_, err = udpConn.Write(packetBytes)
	utils.CheckError(err, "Error sending message to node")
}
