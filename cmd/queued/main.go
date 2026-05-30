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

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)

	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		http.Error(w, "Empty Body", http.StatusBadRequest)
		return
	}

	payload := append([]byte(nil), body...)
	if err := s.d.PushBack(r.Context(), payload); err != nil {
		if errors.Is(err, dq.ErrClosed) {
			http.Error(w, "Server Closed", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusO)
}


func (s *server) handlePop(w http.ResponseWriter, r *http.Request) {

}