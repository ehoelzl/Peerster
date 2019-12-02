package types

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type RoutingTable struct {
	table map[string]*net.UDPAddr
	sync.RWMutex
}

func InitRoutingTable() *RoutingTable {
	return &RoutingTable{
		table: make(map[string]*net.UDPAddr),
	}
}

func (rt *RoutingTable) UpdateRoute(origin string, address *net.UDPAddr, isRouteRumor bool) {
	/*Updates the route to a given origin. Does not print DSDV if routeRumor*/
	rt.Lock()
	defer rt.Unlock()

	rt.table[origin] = address
	if !isRouteRumor {
		fmt.Printf("DSDV %v %v\n", origin, address.String())
	}
}

func (rt *RoutingTable) GetNextHop(destination string) (*net.UDPAddr, bool) {
	/*Returns the recorded nextHop for the given destination*/
	rt.RLock()
	defer rt.RUnlock()
	address, ok := rt.table[destination]
	return address, ok
}

func (rt *RoutingTable) GetJsonString() []byte {
	var origins []string
	for o, _ := range rt.table {
		origins = append(origins, o)
	}
	jsonString, _ := json.Marshal(origins)
	return jsonString
}

func (rt *RoutingTable) GetAllOrigins() map[string]*net.UDPAddr {
	/*Returns the routing Table*/
	rt.RLock()
	defer rt.RUnlock()
	return rt.table
}
