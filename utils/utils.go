package utils

import (
	"fmt"
	"math/rand"
	"time"
)

type PeerTickers struct {
	Tickers map[uint32]chan bool
}

func CheckError(err error, msg string) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}


func CoinFlip() bool {
	return rand.Int()%2 == 0
}



func NewTicker(callback func(), seconds time.Duration) chan bool {
	stop := make(chan bool)
	go func() {
		ticker := time.NewTicker(seconds * time.Second)
		for {
			select {
			case <-stop:
				ticker.Stop()
				return
			case <-ticker.C:
				callback()
			}
		}
	}()
	return stop
}
