package main

import (
	"net/http"
	"github.com/williamntlam/distributed-deque/memory"
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