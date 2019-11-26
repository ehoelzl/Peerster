package utils

import (
	"encoding/hex"
	"fmt"
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
	return rand.Int()%2 == 0
}

func NewTicker(callback func(), seconds time.Duration) chan bool {
	stop := make(chan bool)
	ticker := time.NewTicker(seconds * time.Second)

	go func(tick *time.Ticker, stopChan chan bool) {
		for {
			select {
			case <-stopChan:
				ticker.Stop()
				return
			case <-ticker.C:
				callback()
			}
		}
	}(ticker, stop)
	return stop
}

func ToHex(hash []byte) string {
	return fmt.Sprintf("%x", hash)
}
func ToBytes(hash string) []byte {
	b, err := hex.DecodeString(hash)
	if err != nil {
		return nil
	}
	return b
}
