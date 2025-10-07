package internal

import (
	"net/http"
	"real_time_chat/internal/websocket"
)

type Server struct {
	addr string
}

func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

func (s *Server) Start() error {
	http.HandleFunc("/ws", websocket.WsHandler)
	return http.ListenAndServe(s.addr, nil)
}
