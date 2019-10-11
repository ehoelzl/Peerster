package types

type Peer struct {
	Identifier string
	NextID     uint32
	Messages   map[uint32]*RumorMessage
}

func NewPeer(identifier string) *Peer {
	return &Peer{
		Identifier: identifier,
		NextID:     1,
		Messages:   make(map[uint32]*RumorMessage),
	}
}

func (p *Peer) AddMessage(message *RumorMessage) bool {
	//First check if message ID is == to NextID and corresponds to peer
	if message.Origin != p.Identifier || message.ID != p.NextID { //Previous message or not same peer
		return false
	} else {
		p.Messages[message.ID] = message
		p.NextID += 1
		return true
	}
}
