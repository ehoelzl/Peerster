package types

import (
	"crypto/sha256"
	"fmt"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"sync"
)

var sharedFilesDir = "_SharedFiles"
var chunkSize = 8192

type Chunk struct {
	available bool
	index     int
}
type File struct {
	Filename string
	Metafile []byte
	Size     int64
	Chunks   map[string]*Chunk
}

type Files struct {
	files map[string]*File
	sync.RWMutex
}

func NewFilesStruct() *Files {
	return &Files{
		files: make(map[string]*File),
	}
}

func (fs *Files) IndexNewFile(filename string) bool {
	fs.Lock()
	defer fs.Unlock()

	fileExists, file, fileSize := utils.CheckAndOpen(sharedFilesDir, filename)
	if !fileExists {
		return false
	}

	var metaFile []byte
	buffer := make([]byte, 8192)

	chunkIndex := 0
	chunks := make(map[string]*Chunk)
	for n, err := file.Read(buffer); err == nil; {
		content := buffer[:n]          // Read content
		hash := sha256.Sum256(content) // Hash 8KiB
		metaFile = append(metaFile, hash[:]...)

		chunks[fmt.Sprintf("%x", hash)] = &Chunk{
			available: true,
			index:     chunkIndex,
		}
		chunkIndex += 1
		n, err = file.Read(buffer)
	}

	metaHash := sha256.Sum256(metaFile)
	hashString := fmt.Sprintf("%x", metaHash)

	if _, ok := fs.files[hashString]; ok { // File is already indexed
		return false
	}

	newFile := &File{
		Filename: filename,
		Metafile: metaFile,
		Size:     fileSize,
		Chunks:   chunks,
	}

	fs.files[hashString] = newFile
	log.Printf("File %v indexed\n", hashString)
	return true

}

func (fs *Files) AddEmptyFile(filename string, hash []byte) {
	hashString := utils.ToHex(hash)
	newFile := &File{
		Filename: filename,
		Metafile: nil,
		Size:     0,
		Chunks:   make(map[string]*Chunk),
	}

	fs.Lock()
	defer fs.Unlock()
	if _, ok := fs.files[hashString]; !ok {
		fs.files[hashString] = newFile
	}
}

func (fs *Files) FileExists(hash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	_, ok := fs.files[hashString]
	return ok
}

func (fs *Files) HasMetaFile(hash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	if elem, ok := fs.files[hashString]; ok {
		return elem.Metafile != nil
	}
	return false
}

