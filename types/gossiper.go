package types

import (
	"encoding/json"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"net"
	"strings"
	"time"
)

var budgetLimit uint64 = 32
var matchThreshold = 2

// Gossiper Structure
type Gossiper struct {
	ClientAddress   *net.UDPAddr
	ClientConn      *net.UDPConn
	GossipAddress   *net.UDPAddr
	GossipConn      *net.UDPConn
	IsSimple        bool
	Name            string
	Nodes           *Nodes        // Known nodes (i.e. IP addresses)
	Rumors          *Rumors       // Holds all rumors from known origins
	Routing         *RoutingTable // Routing table
	Files           *Files        // Files (shared and downloaded)
	PrivateMessages []*PrivateMessage
	SearchRequests  *SearchRequests
	FullMatches     *FullMatches
	TLC             *TLC
	PTP 			*PTP
	stubbornTimeout uint64
	hw3ex2          bool
	hopLimit        uint32
	hw3ex3          bool
	ackAll          bool
	hw3ex4          bool
	QSC             *QSC
	Randomness      *Randomness
}

func NewGossiper(uiAddress, gossipAddress, name string, initialPeers string, simple bool, antiEntropy uint, rtimer int, numNodes uint64, stubbornTimeout uint64, hw3ex2 bool, hopLimit uint32, hw3ex3 bool, ackAll bool, hw3ex4 bool) (*Gossiper, bool) {
	// Creates new gossiper with the given parameters
	clientAddr, err := net.ResolveUDPAddr("udp4", uiAddress)
	utils.CheckFatalError(err, fmt.Sprintf("Could not resolve UI Address %v\n", uiAddress))

	clientConn, err := net.ListenUDP("udp4", clientAddr) // Connection to client
	utils.CheckFatalError(err, fmt.Sprintf("Error when opening client UDP channel for %v\n", name))

	gossipAddr, err := net.ResolveUDPAddr("udp4", gossipAddress)
	utils.CheckFatalError(err, fmt.Sprintf("Could not resolve Gossip Address Address %v\n", gossipAddress))

	gossipConn, err := net.ListenUDP("udp4", gossipAddr)
	utils.CheckFatalError(err, fmt.Sprintf("Error when opening gossip UDP channel for %v\n", name))

	randomness, err := getRandomness()
	utils.CheckFatalError(err, fmt.Sprintf("Error getting randomness"))

	log.Printf("Starting gossiper %v\n UIAddress: %v\n GossipAddress %v\n Peers %v\n\n", name, clientAddr, gossipAddr, initialPeers)
	//log.Printf("Randomness is %v", randomness.Point)

	gossiper := &Gossiper{
		ClientAddress:   clientAddr,
		ClientConn:      clientConn,
		GossipAddress:   gossipAddr,
		GossipConn:      gossipConn,
		Name:            name,
		IsSimple:        simple,
		Nodes:           InitNodes(initialPeers),
		Rumors:          InitRumorStruct(name),
		Routing:         InitRoutingTable(),
		Files:           InitFilesStruct(),
		SearchRequests:  InitSearchRequests(),
		FullMatches:     InitFullMatches(),
		TLC:             InitTLCStruct(numNodes, name),
		PTP: 			 InitPTPStruct(numNodes),
		stubbornTimeout: stubbornTimeout,
		hw3ex2:          hw3ex2,
		hopLimit:        hopLimit,
		hw3ex3:          hw3ex3,
		ackAll:          ackAll,
		hw3ex4:          hw3ex4,
		QSC:             InitQSCStruct(),
		Randomness:      randomness,
	}

	// AntiEntropy timer
	utils.NewTicker(func() {
		randomNode, ok := gossiper.Nodes.GetRandomNode(nil)
		if ok {
			gossiper.SendStatusMessage(randomNode)
		}
	}, time.Duration(antiEntropy))

	// Route timer
	if rtimer > 0 {
		utils.NewTicker(gossiper.SendRouteRumor, time.Duration(rtimer))
		gossiper.SendRouteRumor() // Start-up Route rumor
	}
	return gossiper, true
}

/*---------------------------------- Client (CLI/GUI) message handlers ---------------------------------------------*/
func (gp *Gossiper) HandleClientMessage(packetBytes []byte) {
	/*Handles any packet sent by the client (type Message)*/

	// Decode the message
	message := &Message{}

	if err := protobuf.Decode(packetBytes, message); err != nil {
		log.Println("Could not decode packet from client")
		return
	}

	if len(message.Text) > 0 { // Rumor or Private Message
		if message.Destination != nil { // Private Message
			gp.HandleClientPrivateMessage(message)
		} else { // Rumor Message
			gp.HandleClientRumorMessage(message)
		}
	} else if message.File != nil { // File indexing or request
		if len(*message.Request) > 0 { // Request message
			gp.HandleClientFileRequest(message)
		} else { // file indexing
			gp.HandleFileIndexing(message)
		}
	} else if message.Keywords != nil { // Search Request
		gp.HandleClientSearchRequest(message)
	}
}

func (gp *Gossiper) HandleClientPrivateMessage(message *Message) {
	/*Handles private messages*/
	fmt.Printf("CLIENT MESSAGE %v dest %v\n", message.Text, *message.Destination)

	if *message.Destination == gp.Name { // In case someone wants to play it smart
		fmt.Printf("PRIVATE origin %v hop-limit %v contents %v\n", gp.Name, 10, message.Text)
		return
	}

	//Prepare the private message
	pm := &PrivateMessage{
		Origin:      gp.Name,
		ID:          0,
		Text:        message.Text,
		Destination: *message.Destination,
		HopLimit:    gp.hopLimit - 1, // Decrement HopLimit at source node
	}

	gp.SendToNextHop(&GossipPacket{Private: pm}, *message.Destination)
}

func (gp *Gossiper) HandleClientRumorMessage(message *Message) {
	/*Handles a rumor message sent by the client*/
	fmt.Printf("CLIENT MESSAGE %v\n", message.Text)

	if gp.IsSimple { // Simple case
		simpleMessage := &SimpleMessage{
			OriginalName:  gp.Name,
			RelayPeerAddr: gp.GossipAddress.String(),
			Contents:      message.Text,
		}
		gp.Broadcast(&GossipPacket{Simple: simpleMessage}, nil)
	} else { // Rumor case
		rumorMessage := gp.Rumors.CreateNewRumor(gp.Name, message.Text) // Create new rumor
		messageAdded := gp.Rumors.AddRumorMessage(rumorMessage)         // Add it to list
		if messageAdded {                                               //Rumor Only if message was not seen before (i.e. added)
			gp.StartRumormongering(&GossipPacket{Rumor: rumorMessage}, nil, false, true)
		}
	}
}

func (gp *Gossiper) HandleFileIndexing(message *Message) {
	file, indexed := gp.Files.IndexNewFile(*message.File)
	if !indexed {
		return
	}
	if !(gp.hw3ex2 || gp.hw3ex3 || gp.hw3ex4) { // In the case of other exs, do not send TLC
		return
	}
	// Create function that does the TLCMessage (unconfirmed, and start timer that stops when
	tx := TxPublish{
		Name:         file.Filename,
		Size:         file.Size,
		MetafileHash: file.metaHash,
	}
	hasUnconfirmed := gp.TLC.HasUnconfirmed(gp.Name) // Check if we already have an unconfirmed message for the round

	if hasUnconfirmed && (gp.hw3ex3 || gp.hw3ex4) { // Buffer it for next round
		gp.TLC.AddToTxBuffer(&tx)
		return
	}

	gp.SpreadNewBlock(tx)
}

func (gp *Gossiper) SpreadNewBlock(tx TxPublish) {
	var block BlockPublish
	if gp.hw3ex4 {
		block = gp.QSC.GenerateBlock(tx)
	} else {
		block = BlockPublish{Transaction:tx}
	}
	gp.SpreadBlock(block, gp.hw3ex4, 0)

}

func (gp *Gossiper) SpreadBlock(block BlockPublish, randomFitness bool, fitness float32) {
	tlc := gp.Rumors.CreateNewTLCMessage(gp.Name, -1, block, gp.hw3ex3 || gp.hw3ex4, randomFitness, fitness)
	added := gp.Rumors.AddTLCMessage(tlc)
	tx := block.Transaction

	if added {
		gp.TLC.WaitForAcks(tlc.ID) // Add to accumulate ACKS
		packet := &GossipPacket{TLCMessage: tlc}
		// Register callback that runs until we have majority of acks
		callback := func() {
			fmt.Printf("UNCONFIRMED GOSSIP origin %v ID %v file name %v Size %v metahash %v\n",
				tlc.Origin, tlc.ID, tx.Name, tx.Size, utils.ToHex(tx.MetafileHash))
			gp.StartRumormongering(packet, nil, false, true)
		}
		gp.TLC.RegisterTicker(tlc.ID, time.Duration(gp.stubbornTimeout), callback)

		// Start RumorMongering
		callback()
		if gp.hw3ex3 || gp.hw3ex4 {
			gp.TLC.AddUnconfirmed(tlc.ID, gp.Name) // Record that we have a message for this round
			gp.ShouldIncrementRound(tlc)           // Check if we can increment to next round
		}
	}
}

func (gp *Gossiper) HandleClientFileRequest(message *Message) {
	/*Handles a file request done by the client*/
	metaHash := *message.Request // Get the hash of

	if metaHash == nil || gp.Files.IsDownloaded(metaHash) { // Check if it is already downloaded
		log.Println("Cannot download file (maybe already downloaded)")
		return
	}
	// First request the MetaFile
	metaRequest := &DataRequest{
		Origin:    gp.Name,
		HopLimit:  gp.hopLimit - 1,
		HashValue: metaHash,
	}
	var locations []string

	if gp.Files.IsIndexed(metaHash) { // If file Indexed, and destination is specified, update all chunk locations
		if message.Destination != nil {
			gp.Files.UpdateAllChunkLocations(metaHash, *message.Destination)
		}
		if !gp.Files.AllChunksLocationKnown(metaHash) { // We don't know all the locations of chunks
			return
		}
		locations, _ = gp.Files.GetFileChunkLocations(metaHash, 1) // We know there will be a location for this file
	} else { // Otherwise, set destination as specified
		if message.Destination == nil {
			log.Println("Trying to download non-discovered file without destination")
			return
		}
		locations = []string{*message.Destination}
	}

	gp.SendDataRequest(metaHash, *message.File, metaRequest, locations, 0)
}

func (gp *Gossiper) HandleClientSearchRequest(message *Message) {
	budget := *message.Budget

	request := &SearchRequest{
		Origin:   gp.Name,
		Budget:   budget,
		Keywords: utils.ParseKeyWords(*message.Keywords),
	}

	gp.FullMatches.Reset()
	if budget > 0 { // Specified Budget
		gp.PropagateSearchRequest(request, nil)
	} else { // Not Specified, do something else
		gp.InitiateSearch(request)
	}
}

func (gp *Gossiper) InitiateSearch(request *SearchRequest) {
	var budget uint64 = 2
	for budget <= budgetLimit && !gp.FullMatches.AboveThreshold(matchThreshold) {
		request.Budget = budget
		gp.PropagateSearchRequest(request, nil)
		time.Sleep(1 * time.Second) // Sleep one second
		budget *= 2
	}
}

/*---------------------------------- Gossip message handlers  ---------------------------------------------*/

func (gp *Gossiper) HandleGossipPacket(from *net.UDPAddr, packetBytes []byte) {
	/*Handles any GossipPacket received by other nodes*/
	packet := &GossipPacket{}

	if err := protobuf.Decode(packetBytes, packet); err != nil {
		log.Printf("Could not decode GossipPacket from %v\n", from.String())
		return
	}

	gp.Nodes.Add(from) // Add node to list if not in
	if gp.IsSimple {   // Simple case, rebroadcast
		if packet.Simple == nil {
			log.Printf("Empty simple message from %v\n", from.String())
			return
		}
		fmt.Printf("SIMPLE MESSAGE origin %v from %v contents %v \n", packet.Simple.OriginalName, packet.Simple.RelayPeerAddr, packet.Simple.Contents)
		gp.Nodes.Print()

		packet.Simple.RelayPeerAddr = gp.GossipAddress.String()
		gp.Broadcast(&GossipPacket{Simple: packet.Simple}, from)
	} else {                                     // Rumor/Private/File
		if rumor := packet.Rumor; rumor != nil { // RumorMessage
			gp.HandleRumorMessage(from, rumor)
		} else if status := packet.Status; status != nil { //StatusPacket
			gp.HandleStatusPacket(from, status)
		} else if pm := packet.Private; pm != nil { // Private message
			gp.HandlePrivateMessage(from, pm)
		} else if dr := packet.DataRequest; dr != nil { // data request
			gp.HandleDataRequest(from, dr)
		} else if dr := packet.DataReply; dr != nil { // data reply
			gp.HandleDataReply(from, dr)
		} else if sr := packet.SearchRequest; sr != nil { // Search Request
			gp.HandleSearchRequest(from, sr)
		} else if sr := packet.SearchReply; sr != nil {
			gp.HandleSearchReply(from, sr)
		} else if tlc := packet.TLCMessage; tlc != nil {
			gp.HandleTLCMessage(from, tlc)
		} else if ack := packet.Ack; ack != nil {
			gp.HandleTLCAck(from, ack)
		} else if ptp := packet.PTPMessage; ptp != nil {
			gp.HandlePTP(from, ptp)
		} else {
			log.Printf("Empty packet from %v\n", from.String())
			return
		}
	}
}

func (gp *Gossiper) HandleRumorMessage(from *net.UDPAddr, rumor *RumorMessage) {
	/*Handles RumorMessage received by other nodes (route rumor or chat rumor)*/

	fmt.Printf("RUMOR origin %v from %v ID %v contents %v\n", rumor.Origin, from.String(), rumor.ID, rumor.Text)
	gp.Nodes.Print()
	isNewRumor := gp.Rumors.AddRumorMessage(rumor) // Add message to list
	go gp.SendStatusMessage(from)                  // Send back ack

	if isNewRumor { // If message was not seen before, continue rumor mongering to other nodes
		if rumor.Origin != gp.Name { // Double check
			gp.Routing.UpdateRoute(rumor.Origin, from, len(rumor.Text) == 0) // Update routing table (do not print if route rumor)
		}

		except := map[string]struct{}{from.String(): struct{}{}}                 // Monger with other nodes except this one
		gp.StartRumormongering(&GossipPacket{Rumor: rumor}, except, false, true) // Start RumorMongering
		if gp.hw3ex3 || gp.hw3ex4 {
			gp.CheckBufferedTLC() // Check any buffered tlc messages
		}
	}
}

func (gp *Gossiper) HandleStatusPacket(from *net.UDPAddr, status *StatusPacket) {
	/*Handles StatusPacket sent by other nodes (Checks if in sync, or if any of the two has missing rumors)*/

	status.PrintStatusMessage(from) // Print the received status and known nodes
	gp.Nodes.Print()
	lastPacket, isAck := gp.Nodes.CheckTimeouts(from)                      // Check if any timer for the sending node
	missingPacket, isMissing, amMissing := gp.Rumors.CompareStatus(status) // Compare the statuses
	inSync := !(isMissing || amMissing)

	if inSync {
		fmt.Printf("IN SYNC WITH %v\n", from.String())
		if flip := utils.CoinFlip(); isAck && flip { // Check if the StatusPacket is an ACK or if it's just due to AntiEntropy
			except := map[string]struct{}{from.String(): struct{}{}} // Monger with other nodes except this one
			gp.StartRumormongering(lastPacket, except, true, false)
		}
	} else {
		if isMissing { // Means they are missing a message
			gp.SendPacket(missingPacket, from) // Send back missing message
		} else if amMissing { // Means I am missing a message
			gp.SendStatusMessage(from) // I have missing messages
		}
	}
}

func (gp *Gossiper) HandlePrivateMessage(from *net.UDPAddr, pm *PrivateMessage) {
	/*Handles any PrivateMessage sent by another node */
	if pm.HopLimit <= 0 {
		return
	}
	if pm.Destination == gp.Name { //Message has reached destination
		fmt.Printf("PRIVATE origin %v hop-limit %v contents %v\n", pm.Origin, pm.HopLimit, pm.Text)
		gp.PrivateMessages = append(gp.PrivateMessages, pm)
	} else {
		pm.HopLimit -= 1
		gp.SendToNextHop(&GossipPacket{Private: pm}, pm.Destination)
	}
}

func (gp *Gossiper) HandleDataRequest(from *net.UDPAddr, dr *DataRequest) {
	/*Handles a DataRequest (forwards or processes)*/
	if dr.HopLimit <= 0 {
		return
	}
	if dr.Destination == gp.Name { // Request is for this Peerster
		chunk := gp.Files.GetDataChunk(dr.HashValue) // Get the data chunk
		reply := &DataReply{
			Origin:      gp.Name,
			Destination: dr.Origin,
			HopLimit:    gp.hopLimit - 1,
			HashValue:   dr.HashValue,
			Data:        chunk,
		}
		gp.SendToNextHop(&GossipPacket{DataReply: reply}, reply.Destination)
	} else {
		dr.HopLimit -= 1
		gp.SendToNextHop(&GossipPacket{DataRequest: dr}, dr.Destination)
	}
}

func (gp *Gossiper) HandleDataReply(from *net.UDPAddr, dr *DataReply) {
	/*Handles a DataReply (forwards or processes)*/
	if dr.HopLimit <= 0 {
		return
	}
	if dr.Destination == gp.Name {
		metaFileHash, nextChunkIndex := gp.Files.ParseDataReply(dr)
		if nextChunkIndex == 0 {
			return
		}

		nextChunk, hasNext := gp.Files.GetFileChunk(metaFileHash, nextChunkIndex)
		fileName := gp.Files.GetFileName(metaFileHash)
		if hasNext && !nextChunk.available {
			chunkRequest := &DataRequest{
				Origin:    gp.Name,
				HopLimit:  gp.hopLimit - 1,
				HashValue: nextChunk.Hash,
			}
			locations, hasLocations := gp.Files.GetFileChunkLocations(metaFileHash, nextChunkIndex)
			if hasLocations {
				gp.SendDataRequest(metaFileHash, fileName, chunkRequest, locations, nextChunk.index)
			}
		} else if len(fileName) > 0 && gp.Files.IsDownloaded(metaFileHash) {
			fmt.Printf("RECONSTRUCTED file %v\n", fileName)
		}
	} else {
		dr.HopLimit -= 1
		gp.SendToNextHop(&GossipPacket{DataReply: dr}, dr.Destination)
	}
}

func (gp *Gossiper) HandleSearchRequest(from *net.UDPAddr, sr *SearchRequest) {
	/*Handles a received SearchRequest*/
	if sr.Origin == gp.Name { // Ignore requests That echoed back to me
		return
	}
	if added := gp.SearchRequests.AddRequest(sr); !added { // Checks if duplicate requests within 500 ms
		return
	}
	results, ok := gp.Files.SearchFiles(sr.Keywords)
	if ok { // Found matches => Send back reply
		searchReply := &SearchReply{
			Origin:      gp.Name,
			Destination: sr.Origin,
			HopLimit:    gp.hopLimit - 1,
			Results:     results,
		}
		gp.SendToNextHop(&GossipPacket{SearchReply: searchReply}, sr.Origin)
	}
	gp.PropagateSearchRequest(sr, from)
}

func (gp *Gossiper) PropagateSearchRequest(sr *SearchRequest, except *net.UDPAddr) {
	/*Propagates the SearchRequest by Distributing it evenly*/
	sr.Budget -= 1
	if sr.Budget <= 0 { // If budget is 0, do not continue
		return
	}
	nodeBudgets := gp.Nodes.DistributeBudget(sr.Budget, except)
	for addr, budg := range nodeBudgets {
		request := &SearchRequest{
			Origin:   sr.Origin,
			Budget:   budg,
			Keywords: sr.Keywords,
		}
		gp.SendPacket(&GossipPacket{SearchRequest: request}, addr)
	}
}

func (gp *Gossiper) HandleSearchReply(from *net.UDPAddr, sr *SearchReply) {
	if sr.HopLimit <= 0 {
		return
	}

	if sr.Destination == gp.Name {
		sr.Print()
		gp.FullMatches.Add(sr.Results, sr.Origin)        // Add to full match count
		gp.Files.AddSearchResults(sr.Results, sr.Origin) // Add to files
		if gp.FullMatches.AboveThreshold(matchThreshold) {
			fmt.Println("SEARCH FINISHED")
		}
	} else {
		sr.HopLimit -= 1
		gp.SendToNextHop(&GossipPacket{SearchReply: sr}, sr.Destination)
	}
}

func (gp *Gossiper) HandleUnconfirmedTLC(from *net.UDPAddr, tlc *TLCMessage) {
	myRound := gp.TLC.GetMyRound(gp.Name)
	shouldAck := true
	isValid := true // for now

	tx := tlc.TxBlock.Transaction
	if gp.hw3ex3 || gp.hw3ex4 {
		round, shouldIncrement := gp.TLC.GetUnconfirmedMessageRound(tlc)
		if shouldIncrement {
			gp.TLC.IncrementRound(tlc.Origin) // Increment the peer's round
		}
		gp.TLC.AddUnconfirmed(tlc.ID, tlc.Origin) // Record the unconfirmed message
		shouldAck = round >= myRound || gp.ackAll
		if gp.hw3ex4 {
			isValid = gp.QSC.IsValidBlock(tlc.TxBlock) // Check if the Block is confirmed
		}
	}

	fmt.Printf("UNCONFIRMED GOSSIP origin %v ID %v file name %v Size %v metahash %v\n",
		tlc.Origin, tlc.ID, tx.Name, tx.Size, utils.ToHex(tx.MetafileHash))

	isNew := gp.Rumors.AddTLCMessage(tlc) // Add message to list
	go gp.SendStatusMessage(from)         // Send back status

	if isValid && shouldAck {
		ack := &TLCAck{
			Origin:      gp.Name,
			ID:          tlc.ID,
			Destination: tlc.Origin,
			HopLimit:    gp.hopLimit - 1,
		}
		// Send ACK
		fmt.Printf("SENDING ACK origin %v ID %v\n", tlc.Origin, tlc.ID)
		gp.SendToNextHop(&GossipPacket{Ack: ack}, ack.Destination) // Ack message
	}
	// Continue rumor mongering
	except := map[string]struct{}{from.String(): struct{}{}}
	gp.StartRumormongering(&GossipPacket{TLCMessage: tlc}, except, false, true) // Monger the tlcMessage
	if isNew && (gp.hw3ex3 || gp.hw3ex4) {
		gp.CheckBufferedTLC() // Check any buffered tlc messages
	}
}

func (gp *Gossiper) HandleConfirmedTLC(from *net.UDPAddr, tlc *TLCMessage) {
	tx := tlc.TxBlock.Transaction

	fmt.Printf("CONFIRMED GOSSIP origin %v ID %v file name %v Size %v metahash %v\n",
		tlc.Origin, tlc.ID, tx.Name, tx.Size, utils.ToHex(tx.MetafileHash))
	isNew := gp.Rumors.AddTLCMessage(tlc)
	go gp.SendStatusMessage(from)

	except := map[string]struct{}{from.String(): struct{}{}}
	gp.StartRumormongering(&GossipPacket{TLCMessage: tlc}, except, false, true) // Monger the tlcMessage

	if isNew && (gp.hw3ex3 || gp.hw3ex4) { // Only for HW3EX3
		gp.TLC.AddConfirmed(tlc) // Add the Confirmed message to the round it belongs to (depending on the peer)
		gp.ShouldIncrementRound(tlc)
		gp.CheckBufferedTLC() // Check any buffered tlc messages
	}
}

func (gp *Gossiper) HandleTLCMessage(from *net.UDPAddr, tlc *TLCMessage) {
	if tlc.Origin == gp.Name { // Ignore self TLCMessages
		return
	}
	gp.Routing.UpdateRoute(tlc.Origin, from, true) // Never print DSDV

	if gp.hw3ex3 || gp.hw3ex4 { // In ex3, check that vector clock is satisfied
		_, _, notAccepted := gp.Rumors.CompareStatus(tlc.VectorClock) // Accepted if I have seen all messages before it
		if notAccepted {
			gp.SendStatusMessage(from)
			gp.TLC.AddToTLCBuffer(from, tlc) // Add the message to the buffer
			fmt.Printf("Buffer TLC message from %v ID %v\n", tlc.Origin, tlc.ID)
			return
		}
	}
	// Until here, the TLCMessage is either in the same round, or in a previous round
	isUnconfirmed := tlc.Confirmed == -1
	if isUnconfirmed { // Unconfirmed message
		gp.HandleUnconfirmedTLC(from, tlc)
	} else {
		gp.HandleConfirmedTLC(from, tlc)
	}
}

func (gp *Gossiper) CheckBufferedTLC() {
	validTLC := gp.TLC.GetValidTLC(gp.Rumors.CompareStatus)
	for _, tlc := range validTLC {
		if tlc.TLC.Confirmed == -1 {
			gp.HandleUnconfirmedTLC(tlc.Origin, tlc.TLC)
		} else {
			gp.HandleConfirmedTLC(tlc.Origin, tlc.TLC)
		}
	}
}

func (gp *Gossiper) ShouldIncrementRound(tlc *TLCMessage) {
	if gp.TLC.ShouldIncrementRound(gp.Name) {
		currentRound, confirmedMessages := gp.TLC.IncrementMyRound(gp.Name)
		toPrint := PrintTLCMessages(confirmedMessages)
		fmt.Printf("ADVANCING TO round %v BASED ON CONFIRMED MESSAGES %v\n", currentRound, toPrint)
		// Pop the last TX
		if gp.hw3ex3{
			if newTx, hasNew := gp.TLC.PopTx(); hasNew {
				go gp.SpreadNewBlock(*newTx) //
			}
		} else if gp.hw3ex4 {
			switch currentRound % 3 {
			case 1: // Round s+1
				// select confirmed with highest fitness from previous round
				// Set as firstContestant + broadcast
				contestant := GetHighestFitness(confirmedMessages)
				gp.QSC.SetContestant(contestant)
				gp.SpreadBlock(contestant.TxBlock,false, contestant.Fitness) // Spread the block with highest fitness
			case 2: // Round s+2
				// Select confirmed with highest fitness from previous round
				// Select as secondContestant + broadcast
				contestant := GetHighestFitness(confirmedMessages)
				gp.SpreadBlock(contestant.TxBlock,false, contestant.Fitness) // Spread the block with highest fitness
				gp.QSC.AddSecondConfirmations(confirmedMessages)
			case 0:// Round s+3
				/*1. Check that m originates from round s, and threshold witnessed by s+1 (i.e. in confirmedMessages1)
				  2. Check that m was also threshold witnessed by s+2 (i.e. also in confirmedMessages2)
				  3. Does not observe a "more fit" message m' from round s
				*/
				initialMessages := gp.TLC.GetAllConfirmedMessagesFromRound(currentRound - 3)
				gp.QSC.CheckConsensus(initialMessages, gp.TLC.majority)
			}

		}

	}
}

func (gp *Gossiper) HandleTLCAck(from *net.UDPAddr, ack *TLCAck) {
	if ack.HopLimit <= 0 {
		return
	}
	if ack.Destination == gp.Name {
		if isWaiting := gp.TLC.AddAck(ack); !isWaiting { // Check if we were waiting for this ACK ID
			return
		} else if gp.TLC.HasMajority(ack.ID) { // Re broadcast
			gp.TLC.StopTicker(ack.ID)             // Stop the running ticker
			witnesses := gp.TLC.ClearAcks(ack.ID) // Clear acks

			tlcMessage, _ := gp.Rumors.GetTLCMessage(gp.Name, ack.ID) // get the blockPublish corresponding

			reBroadcast := gp.Rumors.CreateNewTLCMessage(gp.Name, int(ack.ID), tlcMessage.TxBlock, gp.hw3ex3 || gp.hw3ex4, false, tlcMessage.Fitness)
			gp.Rumors.AddTLCMessage(reBroadcast)

			fmt.Printf("RE-BROADCAST ID %v WITNESSES %v\n", ack.ID, strings.Join(witnesses, ","))
			gp.StartRumormongering(&GossipPacket{TLCMessage: reBroadcast}, nil, false, true)

			if gp.hw3ex3 || gp.hw3ex4 {
				gp.TLC.AddConfirmed(reBroadcast) // Add message as confirmed
				gp.ShouldIncrementRound(reBroadcast)                    // Check if we need to increment to next round
			}
		}

	} else {
		ack.HopLimit -= 1
		gp.SendToNextHop(&GossipPacket{Ack: ack}, ack.Destination)
	}
}

/*-------------------- Methods used for transferring messages and broadcasting ------------------------------*/

func (gp *Gossiper) SendPacket(packet *GossipPacket, to *net.UDPAddr) {
	/*Constructs and sends a GossipPacket composed of the given arguments*/
	gossipPacket, err := protobuf.Encode(packet)
	if err != nil {
		log.Printf("Error encoding gossipPacket for %v\n", to.String())
		return
	}

	if _, err = gp.GossipConn.WriteToUDP(gossipPacket, to); err != nil {
		log.Printf("Error sending gossipPacket to node %v\n", to.String())
	}

}

func (gp *Gossiper) SendToNextHop(packet *GossipPacket, destination string) bool {
	if nextHop, ok := gp.Routing.GetNextHop(destination); ok {
		gp.SendPacket(packet, nextHop)
		return ok
	}
	return false
}

func (gp *Gossiper) SendStatusMessage(to *net.UDPAddr) {
	/*Sends the current status to the given node*/
	statusPacket := gp.Rumors.GetStatusPacket()
	gp.SendPacket(&GossipPacket{Status: statusPacket}, to)
}

func (gp *Gossiper) Broadcast(packet *GossipPacket, except *net.UDPAddr) {
	/*Broadcasts the packet to all known nodes*/
	nodeAddresses := gp.Nodes.GetAll()
	for _, nodeAddr := range nodeAddresses {
		if nodeAddr.String() != except.String() {
			gp.SendPacket(packet, nodeAddr)
		}
	}
}

func (gp *Gossiper) StartRumormongering(packet *GossipPacket, except map[string]struct{}, coinFlip bool, withTimeout bool) {
	/*Starts the RumorMongering process with the given message*/
	randomNode, ok := gp.Nodes.GetRandomNode(except) // Pick random Node
	if !ok {                                         // Check if could retrieve node
		return
	}
	if coinFlip {
		fmt.Printf("FLIPPED COIN sending rumor to %v\n", randomNode.String())
	} else {
		fmt.Printf("MONGERING with %v\n", randomNode.String())
	}

	if except == nil {
		except = make(map[string]struct{})
	}
	except[randomNode.String()] = struct{}{} // Add node we send to, to the except list

	gp.SendPacket(packet, randomNode) // Send rumor
	if withTimeout {                  // Check if we need to add a timer
		callback := func() { // Callback if not times out
			go gp.StartRumormongering(packet, except, false, true) // Monger with other node
			gp.Nodes.DeleteTicker(randomNode)
		}
		gp.Nodes.StartTicker(randomNode, packet, callback)
	}

}

func (gp *Gossiper) SendRouteRumor() {
	/*Creates a route rumor and sends it to a random Peer*/
	routeRumor := gp.Rumors.CreateNewRumor(gp.Name, "")
	gp.Rumors.AddRumorMessage(routeRumor)
	gp.StartRumormongering(&GossipPacket{Rumor: routeRumor}, nil, false, true)
}

/*-------------------- For File Sharing between gossipers ------------------------------*/

func (gp *Gossiper) SendDataRequest(metaHash []byte, filename string, request *DataRequest, locations []string, chunkId uint64) {
	destination, found := utils.ChooseRandom(locations)
	if !found {
		return
	}
	request.Destination = destination

	callback := func() { go gp.SendDataRequest(metaHash, filename, request, locations, chunkId) } // Callback for ticker

	gp.Files.RegisterRequest(request.HashValue, metaHash, filename, callback) // Register a ticker for the given hash
	if sent := gp.SendToNextHop(&GossipPacket{DataRequest: request}, destination); sent {
		if utils.ToHex(metaHash) == utils.ToHex(request.HashValue) { // This is a MetaFile request
			fmt.Printf("DOWNLOADING metafile of %v from %v\n", filename, destination)
		} else { // This is a chunk request
			fmt.Printf("DOWNLOADING %v chunk %v from %v\n", filename, chunkId, destination)
		}
	}
}

func (gp *Gossiper) MarshallPrivateMessages() []byte {
	response := make(map[string][]string) // Prepare map for response

	for _, m := range gp.PrivateMessages {
		if elem, ok := response[m.Origin]; ok {
			response[m.Origin] = append(elem, m.Text)
		} else {
			response[m.Origin] = []string{m.Text}
		}
	}
	jsonString, _ := json.Marshal(response)
	return jsonString
}
