package main

import (
	"encoding/json"
	"github.com/dedis/protobuf"
	. "github.com/ehoelzl/Peerster/types"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	Address  string
	Gossiper *Gossiper
}

func (s *Server) GetMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonString := s.Gossiper.Rumors.GetRumorsJsonString()
	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetPrivateMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonString := s.Gossiper.MarshallPrivateMessages()
	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetNodeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	var nodesString []string
	nodes := s.Gossiper.Nodes.GetAll()
	for _, n := range nodes {
		nodesString = append(nodesString, n.String())
	}

	jsonString, _ := json.Marshal(nodesString)
	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonString := s.Gossiper.Files.GetJsonString()

	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetOriginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonString := s.Gossiper.Routing.GetJsonString()
	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetIdHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	response := make(map[string]string)
	response["gossipAddress"] = s.Gossiper.GossipAddress.String()
	response["name"] = s.Gossiper.Name
	response["randomnessRound"] = strconv.FormatUint(uint64(HashToNumber(s.Gossiper.Randomness)), 10)
	jsonString, _ := json.Marshal(response)

	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetFullMatches(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonString := s.Gossiper.FullMatches.GetJsonString()

	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) PostMessageHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	var mess Message
	err := decoder.Decode(&mess)
	if err != nil {
		log.Println("Could not decode message from UI")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	isFileIndex := (mess.File != nil) && (mess.Destination == nil) && (mess.Request == nil)
	isFileRequest := (mess.File != nil) && (mess.Request != nil)
	isRumorMessage := (len(mess.Text) > 0) && (mess.Destination == nil)
	isPrivateMessage := (len(mess.Text) > 0) && (mess.Destination != nil)
	isSearchRequest := mess.Keywords != nil
	if isRumorMessage || isPrivateMessage || isFileRequest || isSearchRequest { // Private or rumor message, or file request
		w.WriteHeader(http.StatusOK)
		packetBytes, err := protobuf.Encode(&mess)
		if err != nil {
			log.Println("Could not encode message from UI")
			return
		}

		go s.Gossiper.HandleClientMessage(packetBytes)
	} else if isFileIndex { // File indexing
		_, indexed := s.Gossiper.Files.IndexNewFile(*mess.File)
		if indexed {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("Unknown GUI Command")
	}

}

func (s *Server) PostNodeHandler(w http.ResponseWriter, r *http.Request) {
	//setUpCors(&w, r)

	decoder := json.NewDecoder(r.Body)
	var newNode Message
	err := decoder.Decode(&newNode)

	if err != nil {
		log.Println("Could not decode packet from GUI")
	} else if len(newNode.Text) == 0 {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		nodeAddress, err := net.ResolveUDPAddr("udp4", newNode.Text)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		go s.Gossiper.Nodes.Add(nodeAddress)

	}
}

func NewServer(port string, gossiper *Gossiper) {
	server := &Server{
		Address:  ":" + port,
		Gossiper: gossiper,
	}

	r := mux.NewRouter()

	r.HandleFunc("/message", server.GetMessageHandler).Methods("GET")
	r.HandleFunc("/node", server.GetNodeHandler).Methods("GET")
	r.HandleFunc("/message", server.PostMessageHandler).Methods("POST")
	r.HandleFunc("/node", server.PostNodeHandler).Methods("POST")
	r.HandleFunc("/id", server.GetIdHandler).Methods("GET")
	r.HandleFunc("/origins", server.GetOriginHandler).Methods("GET")
	r.HandleFunc("/private", server.GetPrivateMessageHandler).Methods("GET")
	r.HandleFunc("/files", server.GetFilesHandler).Methods("GET")
	r.HandleFunc("/search", server.GetFullMatches).Methods("GET")
	r.Handle("/", http.FileServer(http.Dir("./frontend")))
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./frontend/")))
	handler := cors.Default().Handler(r)
	srv := &http.Server{
		Handler: handler,
		Addr:    ":" + port,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Printf("Starting server at localhost:%v\n", port)
	log.Printf("WebClient available at http://localhost:%v/\n", port)
	err := srv.ListenAndServe()
	if err != nil {
		log.Println("Error starting server")
	}
}
