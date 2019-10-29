package types

import (
	"fmt"
	"net"
	"sync"
)

type RoutingTable struct {
	Table map[string]*net.UDPAddr
	sync.RWMutex
}

func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		Table: make(map[string]*net.UDPAddr),
	}
}

func (rt *RoutingTable) UpdateRoute(origin string, address *net.UDPAddr, isRouteRumor bool) {
	rt.Lock()
	defer rt.Unlock()
	if elem, ok := rt.Table[origin]; ok && elem.String() == address.String() { // Check if need to update/print
		return
	}
	rt.Table[origin] = address
	if !isRouteRumor {
		fmt.Printf("DSDV %v %v\n", origin, address.String())
	}
}

func (rt *RoutingTable) GetNextHop(destination string) (*net.UDPAddr, bool) {
	rt.RLock()
	defer rt.RUnlock()
	address, ok := rt.Table[destination]
	return address, ok
}