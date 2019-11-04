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
	"time"
)

type Server struct {
	Address  string
	Gossiper *Gossiper
}

func (s *Server) GetMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	rumors := s.Gossiper.Rumors.GetAll()
	response := make(map[string]map[uint32]string) // Prepare map for response

	for identifier, p := range rumors {
		messages := make(map[uint32]string)
		for _, m := range p.Messages {
			if len(m.Text) > 0 {
				messages[m.ID] = m.Text
			}
		}
		if len(messages) > 0 {
			response[identifier] = messages
		}
	}
	jsonString, _ := json.Marshal(response)
	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetPrivateMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	messages := s.Gossiper.PrivateMessages
	response := make(map[string][]string) // Prepare map for response

	for _, m := range messages {
		if elem, ok := response[m.Origin]; ok {
			response[m.Origin] = append(elem, m.Text)
		} else {
			response[m.Origin] = []string{m.Text}
		}
	}
	jsonString, _ := json.Marshal(response)
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
	for n, _ := range nodes {
		nodesString = append(nodesString, n)
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
	messages := s.Gossiper.Files.GetAll()
/*	for h, m := range messages {
		cwd, _ := os.Getwd()
		directory := filepath.Join(cwd, m.Directory, m.Filename)
		messages[h].Directory = directory
	}*/
	jsonString, _ := json.Marshal(messages)
	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetOriginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	var origins []string
	table := s.Gossiper.Routing.GetAllOrigins()
	for o, _ := range table {
		origins = append(origins, o)
	}
	jsonString, _ := json.Marshal(origins)
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
	jsonString, _ := json.Marshal(response)

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

	if mess.File == nil || (mess.File != nil && mess.Destination != nil) { // Private or rumor message, or file request
		w.WriteHeader(http.StatusOK)
		packetBytes, err := protobuf.Encode(&mess)
		if err != nil {
			log.Println("Could not encode message from UI")
			return
		}

		go s.Gossiper.HandleClientMessage(packetBytes)
	} else { // File indexing
		indexed := s.Gossiper.Files.IndexNewFile(*mess.File)
		if indexed {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
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

func NewServer(addr string, gossiper *Gossiper) {
	server := &Server{
		Address:  addr,
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
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./frontend/")))
	handler := cors.Default().Handler(r)
	srv := &http.Server{
		Handler: handler,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Printf("Starting server at %v\n", addr)
	log.Printf("WebClient available at http://%v/\n", addr)
	err := srv.ListenAndServe()
	if err != nil {
		log.Println("Error starting server")
	}
}
