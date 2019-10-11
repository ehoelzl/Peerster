package server

import (
	"encoding/json"
	. "github.com/ehoelzl/Peerster/types"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"time"
)

type Server struct {
	Address  string
	Gossiper *Gossiper
}

type GuiMessage struct {
	Message string
}

func (s *Server) GetMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	messageMap := make(map[string][]string)
	for _, p := range s.Gossiper.Peers {
		if len(p.Messages) > 0 {
			var messages []string
			for _, m := range p.Messages {
				messages = append(messages, m.Text)
			}
			messageMap[p.Identifier] = messages
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
	for _, n := range s.Gossiper.Nodes {
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

	var t GuiMessage
	err := decoder.Decode(&t)

	if err != nil {
		panic(err)
	}
	log.Println(t.Message)
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
