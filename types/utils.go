package types

import (
	"net"
)


func (gp *Gossiper) DeleteTicker(node *net.UDPAddr) {
	if elem, ok := gp.Tickers[node.String()]; ok {
		close(elem)
		delete(gp.Tickers, node.String())
	}
}


func (gp *Gossiper) CheckTickers(sender *net.UDPAddr, status *StatusPacket) bool {
	// Checks the tickers for the given sender and status Message and stops all tickers that have been acked
	if _, ok := gp.Tickers[sender.String()]; ok {
		gp.DeleteTicker(sender)
		return true
	}
	return false
}
