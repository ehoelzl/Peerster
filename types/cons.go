package types

import (
	"crypto/sha256"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"net"
	"sort"
	"sync"
)

type Jam struct {
	Lock sync.RWMutex
	Jammers map[string]string
	ACKS 	map[string]string
	Order 	[]string
	NumberOfNodes uint64
	PROPOSED bool
	MASTER string


}

type DiscussionMessage struct {
	From    *string
	Players *map[string]string
	Order 	*[]string
	MASTER  *string
}


func InitJamStruct(numNodes uint64, gossiperName string) *Jam {
	js := Jam {
		Jammers:       	make(map[string]string),
		NumberOfNodes: 	numNodes,
		PROPOSED: 		false,
		ACKS: 			make(map[string]string),
		Order: 			make([]string, 0),
		MASTER:			"",
	}
	//Save our local name
	hash := sha256.Sum256([]byte(gossiperName))
	uid := utils.ToHex(hash[:])
	js.Jammers[gossiperName] = uid
	return &js
}


func InitPlay(g *Gossiper){
	//First we propagate our name
	dm := DiscussionMessage{
		Players: &g.Jam.Jammers,
		MASTER:  nil,
	}
	g.Broadcast(&GossipPacket{DiscussionMessage:&dm}, nil)
	log.Println("SENDING MY NAME")

}

func (g *Gossiper) HandleDiscussion(from *net.UDPAddr, disc *DiscussionMessage) {

	//We already have all Jammers in our table
	if len(g.Jam.Jammers) == int(g.Jam.NumberOfNodes) {
		if !g.Jam.PROPOSED {
			g.Jam.PROPOSED = true
			go g.ProposeConsensus()
		}
	}

	//First we merge the tables of participants
	if disc.MASTER == nil && disc.Players != nil {
		if len(g.Jam.Jammers) == int(g.Jam.NumberOfNodes) {return}

		for k, v := range *disc.Players{
			if g.Jam.Jammers[k] == "" {
				g.Jam.Jammers[k] = v
			}
		}
		//If we modified it we broadcast our new table
		dm := DiscussionMessage{
			Players: &g.Jam.Jammers,
			MASTER:  nil,
		}
		g.Broadcast(&GossipPacket{DiscussionMessage:&dm}, nil)
	} else
	if disc.Order != nil && g.Jam.PROPOSED == true && g.Name != g.Jam.MASTER {
		g.Jam.Order = *disc.Order
		log.Println("DECIDED", g.Jam.Order, "MASTER", g.Jam.Order[0], "My order", g.Name, g.GetMyOrder())
		ack := &DiscussionMessage{From:&g.Name}
		g.SendPacket(&GossipPacket{DiscussionMessage:ack}, from)
		//g.Broadcast(&GossipPacket{DiscussionMessage:ack}, nil)
	} else
	if disc.From != nil && g.Name == g.Jam.MASTER {
		if g.Jam.ACKS[*disc.From] == "" {
			g.Jam.ACKS[*disc.From] = "true"
		}

		if len(g.Jam.ACKS) == int(g.Jam.NumberOfNodes) {
			log.Println("RECEIVED ALL ACKS, ready to play using ptp")
		}

	}
}

func (g *Gossiper) ProposeConsensus(){
	log.Println("RECEIVED ALL JAMMERS NAME...")
	values := make([]int, 0)
	for _, v := range g.Jam.Jammers {
		values = append(values, int(HashToNumber(v)))
	}
	sort.Ints(values)
	order := make([]string, 0)
	for _, v := range values {
		for name, hash := range g.Jam.Jammers {
			if v == int(HashToNumber(hash)) {
				order = append(order, name)
			}
		}
	}

	//decide
	if order[0] == g.Name {
		g.Jam.MASTER = g.Name
		g.Jam.Order = order
		g.Jam.ACKS[g.Name] = "true"
		log.Println("AT", g.Name,"DECIDED:", order)
		proposition := &DiscussionMessage{Order:&order}
		go g.Broadcast(&GossipPacket{DiscussionMessage: proposition}, nil)
	} else {
		log.Println("WAITING FOR ORDERING")
	}

}


func (g *Gossiper) GetMyOrder() int {
	//TODO : improve or add error
	if g.Jam.Order == nil {
		return int(g.Jam.NumberOfNodes)+1
	}
	for k, v := range g.Jam.Order {
		if v == g.Name {
			return k
		}
	}
	return int(g.Jam.NumberOfNodes)+1
}

func (g *Gossiper) NameToNumber(name string) int{

	return int(HashToNumber(g.Jam.Jammers[name]))

}