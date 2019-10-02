package types

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/ehoelzl/Peerster/utils"
	"net"
)

type Gossiper struct {
	ClientAddress *net.UDPAddr
	ClientConn    *net.UDPConn
	GossipAddress *net.UDPAddr
	GossipConn    *net.UDPConn
	Name          string
	Peers         []*net.UDPAddr
	IsSimple      bool
}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool) *Gossiper {
	clientAddr, err := net.ResolveUDPAddr("udp4", uiAddress)
	clientConn, err := net.ListenUDP("udp4", clientAddr) // Connection to client
	utils.CheckError(err, fmt.Sprintf("Error when opening client UDP channel for %v\n", name))

	gossipAddr, err := net.ResolveUDPAddr("udp4", gossipAddress)
	gossipConn, err := net.ListenUDP("udp4", gossipAddr)
	utils.CheckError(err, fmt.Sprintf("Error when opening gossip UDP channel for %v\n", name))

	fmt.Printf("Starting gossiper %v\n UIAddress: %v\n GossipAddress %v\n Peers %v\n\n", name, clientAddr, gossipAddr, initialPeers)
	return &Gossiper{
		ClientAddress: clientAddr,
		ClientConn:    clientConn,
		GossipAddress: gossipAddr,
		GossipConn:    gossipConn,
		Name:          name,
		Peers:         utils.ParseAddresses(initialPeers),
		IsSimple:      simple,
	}
}

func (gp *Gossiper) StartClientListener() {
	packetBytes := make([]byte, 1024)
	packet := &ClientMessage{}
	for {
		n, _, err := gp.ClientConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from client Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from client Port for %v\n", gp.Name))

		fmt.Println("CLIENT MESSAGE", packet.Contents)
		if gp.IsSimple {
			gp.simpleBroadcast(packet.Contents, gp.Name, nil)
		} else {

		}
	}

}

func (gp *Gossiper) StartGossipListener() {
	packetBytes := make([]byte, 1024)
	packet := &GossipPacket{}
	for {
		n, udpAddr, err := gp.GossipConn.ReadFromUDP(packetBytes)
		utils.CheckError(err, fmt.Sprintf("Error reading from UDP from gossip Port for %v\n", gp.Name))
		err = protobuf.Decode(packetBytes[0:n], packet)
		utils.CheckError(err, fmt.Sprintf("Error decoding packet from gossip Port for %v\n", gp.Name))

		gp.checkNewPeer(udpAddr)
		fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v \n", packet.Simple.OriginalName, packet.Simple.RelayPeerAddr, packet.Simple.Contents)
		utils.PrintPeers(gp.Peers)

		gp.simpleBroadcast(packet.Simple.Contents, packet.Simple.OriginalName, udpAddr)
	}
}

func (gp *Gossiper) simpleBroadcast(contents, originalName string, except *net.UDPAddr) {
	// Double functionality
	gossipMessage := &SimpleMessage{OriginalName: originalName, RelayPeerAddr: gp.GossipAddress.String(), Contents: contents}
	gossipPacket, err := protobuf.Encode(&GossipPacket{Simple: gossipMessage})
	utils.CheckError(err, fmt.Sprintf("Error encoding gossipPacket for %v\n", gp.Name))

	for _, peer := range gp.Peers {
		if peer.String() != except.String() {
			_, err := gp.GossipConn.WriteToUDP(gossipPacket, peer)
			utils.CheckError(err, fmt.Sprintf("Error broadcasting message to node %v\n", peer.String()))
		}
	}
}

func (gp *Gossiper) checkNewPeer(udpAddr *net.UDPAddr) {
	for _, addr := range gp.Peers {
		if addr.String() == udpAddr.String() {
			return
		}
	}
	gp.Peers = append(gp.Peers, udpAddr)
}
