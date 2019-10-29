package utils

import (
	"log"
	"math/rand"
	"time"
)

func CheckError(err error, msg string) {
	if err != nil {
		log.Println(msg)
	}
}

func CheckFatalError(error error, msg string) {
	if error != nil {
		panic(msg)
	}
}


func CoinFlip() bool {
	return rand.Int() % 2 == 0
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

func NewTimoutTicker(callback func(), seconds time.Duration) *time.Ticker {
	// Creates a timeout ticker that ticks only once, and returns the ticker
	ticker := time.NewTicker(seconds * time.Second)
	go func(tick *time.Ticker) {
		for {
			select {
			case <-tick.C:
				//tick.Stop()
				callback()
				return
			}
		}
	}(ticker)
	return ticker
}