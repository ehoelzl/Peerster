package types

import (
	"sync"
	"time"
)

var duplicateThreshold int64 = 500

type SearchRequests struct {
	requests map[int64]*SearchRequest
	sync.RWMutex
}

func (sr *SearchRequests) AddRequest(request *SearchRequest) bool {
	/*Adds the given request to the Map, and deletes requests that are older than 500ms*/
	sr.Lock()
	defer sr.Unlock()

	now := time.Now().UnixNano() / int64(time.Millisecond)
	var todelete []int64
	duplicated := false

	for t, req := range sr.requests {
		if now - t < duplicateThreshold {
			if req.IsDuplicate(request) {
				duplicated = true
			}
		} else {
			todelete = append(todelete, t)
		}
	}

	for _, t := range todelete {
		delete(sr.requests, t)
	}
	if !duplicated {
		sr.requests[now] = request
	}
	return !duplicated
}
