package utils

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
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

func CheckAndOpen(dir string, filename string) (bool, *os.File, int64) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, nil, 0
	}

	filePath := filepath.Join(cwd, dir, filename) // Get filePath
	f, err := os.Open(filePath)                   // Open file

	if os.IsNotExist(err) { // Check existence
		return false, nil, 0
	}
	info, err := f.Stat()

	if err != nil {
		return false, nil, 0
	}

	return !info.IsDir(), f, info.Size()
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