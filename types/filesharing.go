package types

import (
	"crypto/sha256"
	"fmt"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"sync"
)

var sharedFilesDir = "_SharedFiles"
var downloadedDir = "_Downloads"
var chunkSize int64 = 8192
var hashSize int = 32

type Chunk struct {
	available bool
	index     int
	Hash      []byte
}
type File struct {
	Filename  string
	Directory string
	Metafile  []byte
	Size      int64
	Chunks    map[string]*Chunk
	sync.RWMutex
}

type Files struct {
	files map[string]*File
	sync.RWMutex
}

func (f *File) getChunkData(chunk *Chunk) ([]byte, bool) {
	/*Returns the data associated to the given chunk*/
	f.Lock()
	defer f.Unlock()
	/*Returns the corresponding data chunk for the given file*/
	if !chunk.available {
		return nil, false
	}
	exists, file, _ := utils.CheckAndOpen(f.Directory, f.Filename)
	if !exists {
		return nil, false
	}
	_, err := file.Seek(int64(chunk.index)*chunkSize, 0)
	if err != nil {
		return nil, false
	}
	dataChunk := make([]byte, chunkSize)
	_, err = file.Read(dataChunk)
	if err != nil {
		return nil, false
	}
	return dataChunk, true
}

func (f *File) addMetaFile(file []byte) {
	/*Parses the given metafile to get each chunk's hash, and adds the metafile to the file*/
	f.Lock()
	defer f.Unlock()
	f.Metafile = file
	numChunks := len(file) / hashSize // Get the number of chunks
	for i := 0; i < numChunks; i++ {  // Create the chunks with their hashes
		chunkHash := file[i*hashSize : hashSize*(i+1)]
		chunkHashString := utils.ToHex(chunkHash)
		f.Chunks[chunkHashString] = &Chunk{
			available: false,
			index:     i,
			Hash:      chunkHash,
		}
	}
}

func NewFilesStruct() *Files {
	return &Files{
		files: make(map[string]*File),
	}
}

func (fs *Files) IndexNewFile(filename string) bool {
	/*Indexes a new file that should be located under _SharedFiles*/
	fs.Lock()
	defer fs.Unlock()

	fileExists, file, fileSize := utils.CheckAndOpen(sharedFilesDir, filename) // Open file and check existence
	if !fileExists {
		return false
	}

	var metaFile []byte // Create metafile and buffer to read file
	buffer := make([]byte, chunkSize)

	var chunkIndex = 0
	chunks := make(map[string]*Chunk)
	for n, err := file.Read(buffer); err == nil; {
		content := buffer[:n]                   // Read content
		hash := sha256.Sum256(content)          // Hash 8KiB
		metaFile = append(metaFile, hash[:]...) // Append to metaFile

		chunks[fmt.Sprintf("%x", hash)] = &Chunk{ // Add chunk
			available: true,
			index:     chunkIndex,
			Hash:      hash[:],
		}
		chunkIndex += 1
		n, err = file.Read(buffer)
	}

	metaHash := sha256.Sum256(metaFile) // Hash the metaFile
	hashString := fmt.Sprintf("%x", metaHash)

	if _, ok := fs.files[hashString]; ok { // File is already indexed
		return false
	}

	newFile := &File{
		Filename:  filename,
		Directory: sharedFilesDir,
		Metafile:  metaFile,
		Size:      fileSize,
		Chunks:    chunks,
	}

	fs.files[hashString] = newFile
	log.Printf("File %v indexed\n", hashString)
	return true

}

func (fs *Files) AddEmptyFile(filename string, hash []byte) {
	/*Creates an empty File struct to start downloading, puts the Directory as downloadedDir*/
	hashString := utils.ToHex(hash)
	newFile := &File{
		Filename:  filename,
		Metafile:  nil,
		Directory: downloadedDir,
		Size:      0,
		Chunks:    make(map[string]*Chunk),
	}

	fs.Lock()
	defer fs.Unlock()
	if _, ok := fs.files[hashString]; !ok {
		fs.files[hashString] = newFile
	}
}

func (fs *Files) IsIndexed(hash []byte) bool {
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

func (fs *Files) GetDataChunk(hash []byte) ([]byte, bool) {
	/*Returns any chunk of data given it's Hash*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)

	// First check if it corresponds to a metafile
	if elem, ok := fs.files[hashString]; ok { // Means Hash corresponds to metafile
		return elem.Metafile, elem.Metafile != nil
	} else { // Hash maybe corresponds to a chunk of file
		for _, file := range fs.files {
			for h, chunk := range file.Chunks {
				if hashString == h { // Found the chunk
					log.Println("FOund matching hash")
					return file.getChunkData(chunk)
				}
			}
		}
	}
	return nil, false // Could not find the has
}

func (fs *Files) GetFileChunks(hash []byte) (map[string]*Chunk, bool) {
	/*Returns all the chunks of the file*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	if elem, ok := fs.files[hashString]; ok {
		return elem.Chunks, true
	}
	return nil, false
}

func (fs *Files) FileHasChunk(fileHash []byte, chunkHash []byte) bool {
	/*Returns True of the given chunk of the given file is available*/
	fs.RLock()
	defer fs.RUnlock()
	fileHashString := utils.ToHex(fileHash)
	chunkHashString := utils.ToHex(chunkHash)
	if file, ok := fs.files[fileHashString]; ok {
		if chunk, ok := file.Chunks[chunkHashString]; ok {
			return chunk.available
		}
	}

	return false
}

func (fs *Files) ParseDataReply(dr *DataReply) {
	// First check if data is not empty, if empty ?
	if dr.Data == nil {
		return
	}
	dataHash := sha256.Sum256(dr.Data)
	dataHashString := utils.ToHex(dataHash[:])
	hashValueString := utils.ToHex(dr.HashValue)

	if dataHashString != hashValueString {
		log.Println("Got mismatched data and Hash")
		return
	}

	fs.Lock()
	defer fs.Unlock()
	if elem, ok := fs.files[dataHashString]; ok { // Hash corresponds to metafile
		if elem.Metafile == nil {
			elem.addMetaFile(dr.Data)
			log.Println("Added metafile")
			// Need to parse metafile and create all the chunks in the object
			return
		}
	}

}
