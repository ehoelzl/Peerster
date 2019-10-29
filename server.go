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
	messageMap := make(map[string]map[uint32]string)
	for identifier, p := range s.Gossiper.Rumors.Origins {
		numMessages := len(p.Messages)
		if numMessages > 0 {
			messages := make(map[uint32]string)
			for i := 1; i <= numMessages; i++ {
				messages[uint32(i)] = p.Messages[uint32(i)].Text
			}
			messageMap[identifier] = messages
		}
	}
	jsonString, _ := json.Marshal(messageMap)
	_, err := io.WriteString(w, string(jsonString))
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) GetNodeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	var nodesString []string
	for n, _ := range s.Gossiper.Nodes.Addresses {
		nodesString = append(nodesString, n)
	}

	jsonString, _ := json.Marshal(nodesString)
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
	w.WriteHeader(http.StatusOK)
	decoder := json.NewDecoder(r.Body)

	var mess Message
	err := decoder.Decode(&mess)

	if err != nil {
		log.Println("Could not decode message from UI")
		return
	}
	packetBytes, err := protobuf.Encode(&mess)
	if err != nil {
		log.Println("Could not encode message from UI")
		return
	}

	go s.Gossiper.HandleClientMessage(packetBytes)
}

func (s *Server) PostNodeHandler(w http.ResponseWriter, r *http.Request) {
	//setUpCors(&w, r)

	decoder := json.NewDecoder(r.Body)
	var newNode Message
	err := decoder.Decode(&newNode)

	if err != nil {
		log.Println("Could not decode packet from GUI")
	} else {
		nodeAddress, err := net.ResolveUDPAddr("udp4", newNode.Text)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Printf("Could not resolve address %v, added by user\n", newNode.Text)
			return
		} else {
			w.WriteHeader(http.StatusOK)
			go s.Gossiper.Nodes.Add(nodeAddress)
			log.Printf("Added new node at %v\n", nodeAddress)
		}
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
