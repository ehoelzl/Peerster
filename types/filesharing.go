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

type waitingChunk struct {
	metaHash string
	ticker   chan bool
	filename string
}
type File struct {
	Filename  string
	Directory string
	Metafile  []byte
	Size      int64
	Chunks    map[string]*Chunk
	MetaHash  []byte
	sync.RWMutex
}

type Files struct {
	files         map[string]*File
	waitingChunks map[string]*waitingChunk // From chunk hash to waitingChunk
	sync.RWMutex
}

func (f *File) getChunk(index int) (*Chunk, bool) {
	f.RLock()
	defer f.RUnlock()
	for _, chunk := range f.Chunks {
		if chunk.index == index {
			return chunk, true
		}
	}
	return nil, false
}

func (f *File) getChunkData(chunk *Chunk) ([]byte, bool) {
	/*Returns the data associated to the given chunk*/
	f.Lock()
	defer f.Unlock()

	// If chunk not available, cannot send
	if !chunk.available {
		return nil, false
	}
	exists, file, _ := utils.CheckAndOpenRead(f.Directory, f.Filename) // Open the file
	if !exists {
		return nil, false
	}
	_, err := file.Seek(int64(chunk.index)*chunkSize, 0) // Seek to chunk index
	if err != nil {
		return nil, false
	}
	dataChunk := make([]byte, chunkSize)
	n, err := file.Read(dataChunk) // Read into buffer
	if err != nil {
		return nil, false
	}

	if err := file.Close(); err != nil {
		return nil, false
	}
	return dataChunk[:n], true
}

func (f *File) addChunk(chunkHash []byte, chunkData []byte) (*Chunk, bool) {
	f.Lock()
	defer f.Unlock()

	chunkHashString := utils.ToHex(chunkHash)
	if chunk, ok := f.Chunks[chunkHashString]; ok && !chunk.available {
		exists, file, _ := utils.CheckAndOpenWrite(f.Directory, f.Filename) // Open the file
		defer file.Close()
		if !exists {
			return nil, false
		}
		if _, err := file.Write(chunkData); err != nil {
			return nil, false
		}
		f.Chunks[chunkHashString].available = true
		f.Size += int64(len(chunkData))
		nextIndex := chunk.index + 1
		for _, c := range f.Chunks {
			if c.index == nextIndex {
				return c, true
			}
		}
	}
	return nil, false
}

func parseMetaFile(file []byte) map[string]*Chunk {
	/*Parses the given metafile to get each chunk's hash, and adds the metafile to the file*/
	numChunks := len(file) / hashSize // Get the number of chunks
	chunks := make(map[string]*Chunk)
	for i := 0; i < numChunks; i++ { // Create the chunks with their hashes
		chunkHash := file[i*hashSize : hashSize*(i+1)]
		chunkHashString := utils.ToHex(chunkHash)
		chunks[chunkHashString] = &Chunk{
			available: false,
			index:     i,
			Hash:      chunkHash,
		}
	}
	return chunks
}

func InitFilesStruct() *Files {
	return &Files{
		files:         make(map[string]*File),
		waitingChunks: make(map[string]*waitingChunk),
	}
}

func (fs *Files) IndexNewFile(filename string) bool {
	/*Indexes a new file that should be located under _SharedFiles*/
	fs.Lock()
	defer fs.Unlock()

	fileExists, file, fileSize := utils.CheckAndOpenRead(sharedFilesDir, filename) // Open file and check existence
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
	hashString := utils.ToHex(metaHash[:])

	if _, ok := fs.files[hashString]; ok { // File is already indexed
		return false
	}

	newFile := &File{
		Filename:  filename,
		Directory: sharedFilesDir,
		Metafile:  metaFile,
		Size:      fileSize,
		Chunks:    chunks,
		MetaHash:  metaHash[:],
	}

	fs.files[hashString] = newFile
	log.Printf("File %v indexed\n", hashString)
	return true

}

func (fs *Files) IsIndexed(hash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	_, ok := fs.files[hashString]
	return ok
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
					return file.getChunkData(chunk)
				}
			}
		}
	}
	return nil, false // Could not find the has
}

func (fs *Files) RegisterRequest(chunkHash []byte, metaHash []byte, filename string, callback func()) {
	/*Registers a ticker for the given request, that will periodically call the callback function until stopped*/
	fs.Lock()
	defer fs.Unlock()

	chunkHashString := utils.ToHex(chunkHash)
	metaHashString := utils.ToHex(metaHash)
	ticker := utils.NewTicker(callback, 5) // Ticker for 5 seconds
	waiting := &waitingChunk{
		metaHash: metaHashString,
		ticker:   ticker,
		filename: filename,
	}
	fs.waitingChunks[chunkHashString] = waiting
}

func (fs *Files) ParseDataReply(dr *DataReply) (*File, *Chunk, bool) {
	/*Parses a reply coming for a requested document. Returns a (*Chunk, bool) to indicate the next chunk*/
	// First check if data is not empty, if empty discard
	fs.Lock()
	defer fs.Unlock()

	var nextChunk *Chunk = nil
	var file *File = nil
	hasNext := false
	hashValueString := utils.ToHex(dr.HashValue) // Hex hash in reply

	if waiting, ok := fs.waitingChunks[hashValueString]; ok { // Was waiting for this hash
		waiting.ticker <- true // stop the running ticker
		if dr.Data == nil {
			return nil, nil, false
		}
		dataHash := sha256.Sum256(dr.Data)         // Compute the hash of the sent data
		dataHashString := utils.ToHex(dataHash[:]) // Hex hash of data

		if dataHashString != hashValueString { // Drop message if incorrect data
			return nil, nil, false
		}

		if waiting.metaHash == hashValueString { // This is a requested metaFile
			nextChunk, hasNext = fs.createFile(waiting.filename, dr.HashValue, dr.Data)
		} else { // This is a requested chunk
			if file, ok := fs.files[waiting.metaHash]; ok {
				nextChunk, hasNext = file.addChunk(dr.HashValue, dr.Data)
			}
		}
		file = fs.files[waiting.metaHash]
		delete(fs.waitingChunks, hashValueString)
	}

	return file, nextChunk, hasNext
}

func (fs *Files) createFile(filename string, metaHash []byte, metaFile []byte) (*Chunk, bool) {
	/*Creates an empty File struct to start downloading, puts the Directory as downloadedDir*/
	hashString := utils.ToHex(metaHash)
	if _, ok := fs.files[hashString]; ok { // Check if we don't have the file already
		return nil, false
	}

	chunks := parseMetaFile(metaFile)
	newFile := &File{
		Filename:  filename,
		Metafile:  metaFile,
		Directory: downloadedDir,
		Size:      0,
		Chunks:    chunks,
		MetaHash:  metaHash,
	}
	fs.files[hashString] = newFile
	utils.CreateEmptyFile(downloadedDir, filename)
	for _, chunk := range chunks {
		if chunk.index == 0 {
			return chunk, true
		}
	}
	return nil, false
}
