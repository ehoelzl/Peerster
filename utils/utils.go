package utils

import (
	"fmt"
	"net"
	"strings"
)

func CheckError(err error, msg string) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}

func PrintPeers(peers []*net.UDPAddr) {
	var stringAddresses []string
	for _, peer := range peers {
		stringAddresses = append(stringAddresses, peer.String())
	}
	fmt.Printf("PEERS %v\n", strings.Join(stringAddresses, ","))
}

func ParseAddresses(addresses string) []*net.UDPAddr {
	var peerAddresses []*net.UDPAddr
	if len(addresses) > 0 {
		for _, peer := range strings.Split(addresses, ",") {
			peerAddr, err := net.ResolveUDPAddr("udp4", peer)
			if err != nil {
				fmt.Printf("Could not resolve peer address at %v\n")
			} else {
				peerAddresses = append(peerAddresses, peerAddr)
			}
		}
	}
	return peerAddresses
}

