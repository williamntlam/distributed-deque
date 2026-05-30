package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	dq "github.com/williamntlam/distributed-deque"
	"github.com/williamntlam/distributed-deque/memory"
)

const (
	addr         = ":8080"
	maxBodyBytes = 1 << 20 // 1 MiB
)

type server struct {
	d *memory.MemoryDeque
}

func newServer() *server {
	return &server{d: memory.NewMemoryDeque()}
}

func (s *server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /push", s.handlePush)
	mux.HandleFunc("GET /pop", s.handlePop)
}

func (s *server) handlePush(w http.ResponseWriter, r *http.Request) {

}


func (s *server) handlePop(w http.ResponseWriter, r *http.Request) {

}