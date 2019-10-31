package types

import (
	"crypto/sha256"
	"github.com/ehoelzl/Peerster/utils"
	"golang.org/x/tools/go/ssa/interp/testdata/src/fmt"
	"math"
	"sync"
)

var sharedFilesDir = "_SharedFiles"
var chunkSize = 8192

type File struct {
	Filename string
	Metafile string
	Size     int64
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
	fileExists, file, fileSize := utils.CheckAndOpen(sharedFilesDir, filename)
	if !fileExists {
		return false
	}
	numChunks := int(math.Ceil(float64(fileSize) / float64(chunkSize)))
	meta := make([]byte, 32*numChunks)
	metaIndex := 0

	buffer := make([]byte, 8192)
	for n, err := file.Read(buffer); err == nil; {
		content := buffer[:n] // Read content
		hash := sha256.Sum256(content)
		copy(meta[metaIndex:metaIndex+32], hash[:])
		metaIndex += 32
		n, err = file.Read(buffer)
	}

	metaSaved, metaFile, metaHash := utils.SaveMetaFile(sharedFilesDir, filename, meta)

	hashString := fmt.Sprint("%x", metaHash)
	if !metaSaved {
		return false
	}

	fs.Lock()
	defer fs.Unlock()
	if _, ok := fs.files[hashString]; ok { // File is already indexed
		return false
	}

	newFile := &File{
		Filename: filename,
		Metafile: metaFile,
		Size:     fileSize,
	}

	fs.files[hashString] = newFile
	return true

}
