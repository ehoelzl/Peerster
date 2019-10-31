package utils

import (
	"crypto/sha256"
	"io"
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

func SaveMetaFile(dir, filename string, contents []byte) (bool, string, []byte) {
	cwd, err := os.Getwd() // Get WD
	if err != nil {
		return false, "", nil
	}

	extension := filepath.Ext(filename)                   // Get length of extension
	filename = filename[0 : len(filename)-len(extension)] // Remove extension
	filename += ".meta"

	filePath := filepath.Join(cwd, dir, filename) // Get filePath

	f, err := os.Create(filePath) // Create the file

	if err != nil {
		return false, "", nil
	}
	defer f.Close()

	_, err = f.Write(contents)
	if err != nil {
		return false, "", nil
	}

	f, err = os.Open(filePath) // Re-open the file
	if err != nil {
		return false, "", nil
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, "", nil
	}

	metaHash := h.Sum(nil)
	return true, filename, metaHash
}
