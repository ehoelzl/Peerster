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
	dest := flag.String("dest", "", "destination for the private message; can be omitted")
	msg := flag.String("msg", "", "message to be sent; if the -dest flag is present, this is a private message, otherwise it's a rumor message")
	file := flag.String("file", "", "file to be indexed by the gossiper")

	flag.Parse()

	if len(*msg) == 0 && len(*file) == 0{
		log.Println("Cannot send empty message")
	}
	if len(*dest) == 0 {
		dest = nil
	}

	if len(*file) == 0 {
		file = nil
	}

	message := Message{Text: *msg, Destination: dest, File: file}
	packetBytes, err := protobuf.Encode(&message)

	utils.CheckError(err, "Error encoding message")

	address := "127.0.0.1:" + *uiPort
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	udpConn, _ := net.DialUDP("udp4", nil, udpAddr)
	_, err = udpConn.Write(packetBytes)
	utils.CheckError(err, "Error sending message to node")
}
