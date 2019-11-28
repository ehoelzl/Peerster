package main

import (
	"flag"
	"fmt"
	"github.com/dedis/protobuf"
	. "github.com/ehoelzl/Peerster/types"
	"github.com/ehoelzl/Peerster/utils"
	"net"
	"os"
)

func main() {
	uiPort := flag.String("UIPort", "8080", "port for the UI client (default \"8080\")")
	dest := flag.String("dest", "", "destination for the private message; can be omitted")
	msg := flag.String("msg", "", "message to be sent; if the -dest flag is present, this is a private message, otherwise it's a rumor message")
	file := flag.String("file", "", "file to be indexed by the gossiper")
	request := flag.String("request", "", "request a chunk or metafile of this hash")
	budget := flag.Uint64("budget", 0, "budget for search request")
	keywords := flag.String("keywords", "", "keywords for search request")
	flag.Parse()

	// Must specify UIPort
	if len(*uiPort) == 0 {
		fmt.Println("ERROR (Please specify UIPort)")
		os.Exit(1)
	}

	isRumor := (len(*msg) > 0) && (len(*dest) == 0) && (len(*file) == 0) && (len(*request) == 0) && (len(*keywords) == 0) && (*budget == 0)
	isPrivate := (len(*msg) > 0) && (len(*dest) > 0) && (len(*file) == 0) && (len(*request) == 0) && (len(*keywords) == 0) && (*budget == 0)
	isFileIndex := (len(*msg) == 0) && (len(*dest) == 0) && (len(*file) > 0) && (len(*request) == 0) && (len(*keywords) == 0) && (*budget == 0)
	isFileRequest := (len(*msg) == 0) && (len(*file) > 0) && (len(*request) > 0) && (len(*keywords) == 0) && (*budget == 0)
	isSearchRequest := (len(*msg) == 0) && (len(*dest) == 0) && (len(*file) == 0) && (len(*request) == 0) && (len(*keywords) > 0)

	validCombination := isRumor || isPrivate || isFileIndex || isFileRequest || isSearchRequest
	if !validCombination {
		fmt.Println("ERROR (Bad argument combination)")
		os.Exit(1)
	}

	if len(*dest) == 0 {
		dest = nil
	}
	if len(*file) == 0 {
		file = nil
	}

	if len(*keywords) == 0 {
		keywords = nil
	}

	var requestBytes []byte

	if len(*request) == 0 {
		requestBytes = nil
	} else {
		requestBytes = utils.ToBytes(*request)
		if requestBytes == nil || len(requestBytes) < 32 {
			fmt.Println("ERROR (Unable to decode hex hash)")
		}
	}

	message := Message{Text: *msg, Destination: dest, File: file, Request: &requestBytes, Keywords: keywords, Budget: budget}
	packetBytes, err := protobuf.Encode(&message)

	utils.CheckError(err, "Error encoding message")

	address := "127.0.0.1:" + *uiPort
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	udpConn, _ := net.DialUDP("udp4", nil, udpAddr)
	_, err = udpConn.Write(packetBytes)
	utils.CheckError(err, "Error sending message to node")
}
