package types

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
)

var sharedFilesDir = "_SharedFiles"
var downloadedDir = "_Downloads"
var chunkSize int64 = 8192
var hashSize int = 32

type Chunk struct {
	available bool
	index     uint64
	Hash      []byte
}

type File struct {
	Filename       string
	Path           string
	metaFile       []byte
	Size           int64
	chunks         map[string]*Chunk
	chunkLocations map[uint64]map[string]struct{}
	IsDownloaded   bool
	isComplete     bool
	chunkCount     uint64
	metaHash       []byte
}

type requestedChunk struct { // Structure for a requested chunk that has not yet been received
	metaHash string    //Has of the associated metaFile
	ticker   chan bool // Channel for running ticker
	filename string    // Filename
}

type Files struct {
	files           map[string]*File
	requestedChunks map[string]*requestedChunk // From chunk hash to requestedChunk
	sync.RWMutex
}

func (f *File) getChunkData(chunk *Chunk) []byte {
	/*Returns the data associated to the given chunk (Opens the file, reads the chunk and returns it)*/
	// If chunk not available, cannot send
	if !chunk.available {
		return nil
	}
	exists, file, _ := utils.CheckAndOpenRead(f.Path) // Open the file
	if !exists {
		return nil
	}
	_, err := file.Seek(int64(chunk.index-1)*chunkSize, 0) // Seek to chunk index
	if err != nil {
		return nil
	}
	dataChunk := make([]byte, chunkSize)
	n, err := file.Read(dataChunk) // Read into buffer
	if err != nil {
		return nil
	}

	if err := file.Close(); err != nil {
		return nil
	}

	return dataChunk[:n]
}

func (f *File) addChunk(chunkHash []byte, chunkData []byte) (uint64, bool) {
	/*Adds a given chunk to the file (opens and appends the bytes). Returns the next chunk, and a flag indicating whether there is one*/

	chunkHashString := utils.ToHex(chunkHash)
	if chunk, ok := f.chunks[chunkHashString]; ok && !chunk.available { // Check that the chunk was not received already
		file, exists := utils.CheckAndOpenWrite(f.Path) // Open the file
		if !exists {
			return 0, false
		}

		if _, err := file.Seek(int64(chunk.index-1)*chunkSize, 0); err != nil { // Seek to index
			return 0, false
		}
		if _, err := file.Write(chunkData); err != nil { // Write chunk at index
			return 0, false
		}
		chunk.available = true          // Mark chunk as available
		f.Size += int64(len(chunkData)) // Increase the Size

		isDownloaded := true
		for _, c := range f.chunks {
			if !c.available {
				isDownloaded = false
				break
			}
		}

		if isDownloaded { // If all chunks have been downloaded, truncate at Size
			if err := file.Truncate(f.Size); err != nil {
				log.Println("Could not truncate file")
			}
			f.IsDownloaded = true // Mark file as downloaded
		}
		if err := file.Close(); err != nil { // Close the file
			return 0, false
		}
		return chunk.index + 1, true

	}
	return 0, false
}

func (f *File) updateChunkLocations(chunkMap []uint64, origin string) {
	for _, chunkIndex := range chunkMap {
		if _, ok := f.chunkLocations[chunkIndex]; !ok {
			f.chunkLocations[chunkIndex] = make(map[string]struct{})
		}
		f.chunkLocations[chunkIndex][origin] = struct{}{}

	}
	if uint64(len(f.chunkLocations)) == f.chunkCount {
		f.isComplete = true
	}
}

func (f *File) deleteChunkLocation(chunkHash string, origin string) {
	chunk, present := f.chunks[chunkHash]
	if !present {
		return
	}
	delete(f.chunkLocations[chunk.index], origin)

	if len(f.chunkLocations[chunk.index]) == 0 {
		delete(f.chunkLocations, chunk.index)
		f.isComplete = false
	}
}

func (f *File) searchResult() *SearchResult {
	/*Transforms a file into SearchResult*/
	var chunkMap []uint64
	for _, c := range f.chunks {
		if c.available {
			chunkMap = append(chunkMap, c.index)
		}
	}
	if len(chunkMap) == 0 {
		return nil
	}
	sort.SliceStable(chunkMap, func(i, j int) bool { return chunkMap[i] < chunkMap[j] })

	chunkCount := len(f.metaFile) / hashSize
	return &SearchResult{
		FileName:     f.Filename,
		MetafileHash: f.metaHash,
		ChunkMap:     chunkMap,
		ChunkCount:   uint64(chunkCount),
	}
}

func (f *File) GetBlockPublish() BlockPublish {
	txPublish := TxPublish{
		Name:         f.Filename,
		Size:         f.Size,
		MetaFileHash: f.metaHash,
	}

	return BlockPublish{
		Transaction: txPublish,
	}
}

func InitFilesStruct() *Files {
	return &Files{
		files:           make(map[string]*File),
		requestedChunks: make(map[string]*requestedChunk),
	}
}

/*==================================== File Indexing ====================================*/

func (fs *Files) IndexNewFile(filename string) (*File, bool) {
	/*Indexes a new file that should be located under _SharedFiles*/
	fs.Lock()
	defer fs.Unlock()

	filePath := utils.GetAbsolutePath(sharedFilesDir, filename)
	fileExists, file, fileSize := utils.CheckAndOpenRead(filePath) // Open file and check existence
	if !fileExists {
		return nil, false
	}
	metaFile, chunks := createMetaFile(file)
	metaHash := sha256.Sum256(metaFile) // Hash the metaFile
	hashString := utils.ToHex(metaHash[:])

	if _, ok := fs.files[hashString]; ok { // File is already indexed
		return nil, false
	}

	newFile := &File{
		Filename:     filename,
		Path:         filePath,
		metaFile:     metaFile,
		Size:         fileSize,
		chunks:       chunks,
		metaHash:     metaHash[:],
		IsDownloaded: true,
	}

	fs.files[hashString] = newFile
	log.Printf("File %v indexed\n", hashString)
	return newFile, true

}

func createMetaFile(file *os.File) ([]byte, map[string]*Chunk) {
	/*Reads a file and chunks it into 8KiB chunks, hashes them and concatenates them to an array*/
	chunks := make(map[string]*Chunk)
	buffer := make([]byte, chunkSize)

	var chunkIndex uint64 = 1
	var metaFile []byte

	for n, err := file.Read(buffer); err == nil; {
		content := buffer[:n]                   // Read content
		hash := sha256.Sum256(content)          // Hash 8KiB
		metaFile = append(metaFile, hash[:]...) // Append to metaFile

		chunks[utils.ToHex(hash[:])] = &Chunk{ // Add chunk
			available: true,
			index:     chunkIndex,
			Hash:      hash[:],
		}
		chunkIndex += 1
		n, err = file.Read(buffer)
	}
	return metaFile, chunks
}

func (fs *Files) IsIndexed(metaFileHash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	if metaFileHash == nil {
		return false
	}
	_, indexed := fs.files[utils.ToHex(metaFileHash)]
	return indexed
}

func (fs *Files) GetFileChunk(metaFileHash []byte, chunkIndex uint64) (*Chunk, bool) {
	fs.RLock()
	defer fs.RUnlock()
	if file, ok := fs.files[utils.ToHex(metaFileHash)]; ok {
		for _, chunk := range file.chunks {
			if chunk.index == chunkIndex {
				return chunk, true
			}
		}
	}
	return nil, false
}

func (fs *Files) GetFileName(metaFileHash []byte) string {
	fs.RLock()
	defer fs.RUnlock()
	if file, ok := fs.files[utils.ToHex(metaFileHash)]; ok {
		return file.Filename
	}
	return ""
}

/*==================================== File Download ====================================*/

func (fs *Files) IsDownloaded(metaFileHash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	if file, ok := fs.files[utils.ToHex(metaFileHash)]; ok {
		return file.IsDownloaded
	}
	return false
}

func (fs *Files) GetDataChunk(hash []byte) []byte {
	/*Returns any chunk of data given it's Hash*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)

	// First check if it corresponds to a metafile
	if elem, ok := fs.files[hashString]; ok { // Means Hash corresponds to metafile
		return elem.metaFile
	} else { // Hash maybe corresponds to a chunk of file
		for _, file := range fs.files {
			if chunk, ok := file.chunks[hashString]; ok { // Found the chunk
				return file.getChunkData(chunk)
			}
		}
	}
	return nil // Could not find the hash
}

func (fs *Files) RegisterRequest(chunkHash []byte, metaHash []byte, filename string, callback func()) {
	/*Registers a ticker for the given request, that will periodically call the callback function until stopped*/
	fs.Lock()
	defer fs.Unlock()

	chunkHashString := utils.ToHex(chunkHash)
	// First check if there are already tickers running for this chunk, delete them and stop them
	if req, ok := fs.requestedChunks[chunkHashString]; ok {
		req.ticker <- true
		delete(fs.requestedChunks, chunkHashString)
	}
	metaHashString := utils.ToHex(metaHash)
	ticker := utils.NewTicker(callback, 5) // Ticker for 5 seconds
	requested := &requestedChunk{
		metaHash: metaHashString,
		ticker:   ticker,
		filename: filename,
	}
	fs.requestedChunks[chunkHashString] = requested // Add the new ticker
}

func (fs *Files) ParseDataReply(dr *DataReply) ([]byte, uint64) {
	/*Parses a reply coming for a requested document. Returns a (metaFileHash, nextChunkIndex)*/
	fs.Lock()
	defer fs.Unlock()

	hashValueString := utils.ToHex(dr.HashValue) // Hex hash in reply

	if requested, ok := fs.requestedChunks[hashValueString]; ok { // Was requested for this hash
		requested.ticker <- true                            // stop the running ticker
		defer delete(fs.requestedChunks, hashValueString)   // Delete the requested chunks
		isMetaFile := requested.metaHash == hashValueString // Means that the reply is a MetaFile

		var nextChunkIndex uint64
		if len(dr.Data) == 0 && !isMetaFile { // Requested chunk is not available at dr.Origin
			file := fs.files[requested.metaHash]                 // Get the corresponding file
			file.deleteChunkLocation(hashValueString, dr.Origin) // Mark this chunk as not being in that location

			chunk := file.chunks[hashValueString]
			nextChunkIndex = chunk.index
		} else if len(dr.Data) > 0 && utils.CheckDataHash(dr.Data, hashValueString) { // Data not empty, and hash matches
			if isMetaFile {
				created := fs.createDownloadFile(requested.filename, dr.HashValue, dr.Data, dr.Origin)
				if created {
					nextChunkIndex = 1
				}
			} else if file, ok := fs.files[requested.metaHash]; ok { // This is a requested chunk
				nextIndex, chunkAdded := file.addChunk(dr.HashValue, dr.Data)
				if chunkAdded {
					nextChunkIndex = nextIndex
				}
			}
		}
		return utils.ToBytes(requested.metaHash), nextChunkIndex
	}
	return nil, 0
}

func (fs *Files) createDownloadFile(filename string, metaHash []byte, metaFile []byte, origin string) (bool) {
	/*Creates an empty File struct to start downloading, puts the Path as downloadedDir*/
	hashString := utils.ToHex(metaHash)
	file, filePresent := fs.files[hashString]
	if filePresent && file.metaFile != nil { // Check if we don't have the file already
		return false
	}

	chunks := parseMetaFile(metaFile) // Parse all the chunks
	filePath := utils.GetAbsolutePath(downloadedDir, filename)

	if !filePresent { // Direct download (HW2)
		chunkCount := uint64(len(chunks))
		file = &File{
			chunkCount:     chunkCount,
			metaHash:       metaHash,
			chunkLocations: make(map[uint64]map[string]struct{}),
		}
		var chunkMap []uint64
		for i := uint64(1); i <= chunkCount; i++ {
			chunkMap = append(chunkMap, i)
		}
		file.updateChunkLocations(chunkMap, origin) // Assign the origin as having all chunks
		fs.files[hashString] = file
	}
	file.metaFile = metaFile
	file.Path = filePath
	file.chunks = chunks
	file.Filename = filename

	utils.CreateEmptyFile(filePath, int64(len(chunks))*chunkSize)

	return true
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
			index:     uint64(i + 1),
			Hash:      chunkHash,
		}
	}
	return chunks
}

/*==================================== File Search ====================================*/

func (fs *Files) AllChunksLocationKnown(metaFileHash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	if file, ok := fs.files[utils.ToHex(metaFileHash)]; ok {
		return file.isComplete
	}
	return false
}

func (fs *Files) GetFileChunkLocations(metaFileHash []byte, index uint64) ([]string, bool) {
	fs.RLock()
	defer fs.RUnlock()
	if file, ok := fs.files[utils.ToHex(metaFileHash)]; ok {
		if locations, ok := file.chunkLocations[index]; ok {
			return utils.MapToSlice(locations), true
		}
	}
	return nil, false
}

func (fs *Files) UpdateAllChunkLocations(metaFileHash []byte, origin string) {
	/*Updates the location of all chunks to one origin*/
	fs.Lock()
	defer fs.Unlock()
	if file, ok := fs.files[utils.ToHex(metaFileHash)]; ok {
		var chunkMap []uint64
		for i := uint64(1); i <= file.chunkCount; i++ {
			chunkMap = append(chunkMap, i)
		}
		file.updateChunkLocations(chunkMap, origin)
	}

}

func (fs *Files) SearchFiles(keywords []string) ([]*SearchResult, bool) {
	/*Searches for files that contain the given keywords*/
	fs.RLock()
	defer fs.RUnlock()

	matches := make(map[string]bool)
	var results []*SearchResult
	for _, k := range keywords {
		if len(k) == 0 { // Double check
			continue
		}
		for _, f := range fs.files {
			if _, ok := matches[f.Filename]; !ok && strings.Contains(f.Filename, k) { // Match for this name
				matches[f.Filename] = true
				result := f.searchResult()
				if result != nil {
					results = append(results, result)
				}
			}
		}
	}
	return results, len(results) > 0
}

func (fs *Files) AddSearchResults(sr []*SearchResult, origin string) {
	/*Adds the search results to the List of files, by adding the locations of each chunk*/
	if sr == nil || len(sr) == 0 {
		return
	}
	fs.Lock()
	defer fs.Unlock()

	for _, res := range sr {
		metaFileHash := utils.ToHex(res.MetafileHash)
		file, exists := fs.files[metaFileHash]
		if !exists { // New file
			file = &File{
				chunkCount:     res.ChunkCount,
				metaHash:       res.MetafileHash,
				chunkLocations: make(map[uint64]map[string]struct{}),
			}
			fs.files[metaFileHash] = file
		}
		file.updateChunkLocations(res.ChunkMap, origin) // Update the locations of each chunk

	}

}

/*==================================== Server functions ====================================*/

func (fs *Files) GetJsonString() []byte {
	fs.RLock()
	defer fs.RUnlock()
	jsonString, err := json.Marshal(fs.files)
	if err != nil {
		log.Println("Could not marshall Files")
		return nil
	}
	return jsonString
}
