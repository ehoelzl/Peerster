package server

import (
	"encoding/json"
	"fmt"
	. "github.com/ehoelzl/Peerster/types"
	"github.com/gorilla/mux"
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
	messageMap := make(map[string][]string)
	for identifier, p := range s.Gossiper.Peers.Peers {
		if len(p.Messages) > 0 {
			var messages []string
			for _, m := range p.Messages {
				messages = append(messages, m.Text)
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
	for _, n := range s.Gossiper.Nodes.Addresses {
		nodesString = append(nodesString, n.String())
	}

	jsonString, _ := json.Marshal(nodesString)
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
		panic(err)
	}
	go s.Gossiper.HandleClientMessage(&mess)
}

func (s *Server) PostNodeHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	var newNode Message
	err := decoder.Decode(&newNode)

	if err != nil {
		fmt.Println("Could not encode message from client")
	} else {
		nodeAddress, err := net.ResolveUDPAddr("udp4", newNode.Text)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Printf("Could not resolve address %v, added by user\n", newNode.Text)
		} else {
			w.WriteHeader(http.StatusOK)
			go s.Gossiper.Nodes.AddNode(nodeAddress)
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

	srv := &http.Server{
		Handler: r,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Printf("Starting server at %v\n", addr)
	log.Fatal(srv.ListenAndServe())
}
