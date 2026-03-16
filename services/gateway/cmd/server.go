package main

import (
	"log"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/auth"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/friends"
	authpb "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/pb/auth"
	friendspb "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/pb/friends"
	websocketproxy "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/websocket_proxy"
)

type Server struct {
	addr             string
	websocketURL     string
	authServiceCli   authpb.AuthServiceClient
	friendServiceCli friendspb.FriendsServiceClient
}

func NewServer(addr, authAddr, friendAddr, websocketURL string) *Server {
	authService, err := auth.NewAuthServiceClient(authAddr)
	if err != nil {
		log.Fatal("error connecting with auth service", err)
	}

	friendsService, err := friends.NewFriendsServiceClient(friendAddr)
	if err != nil {
		log.Fatal("error connecting with friends service", err)
	}

	log.Println("Gateway connected with Authentication Service")
	log.Println("Gateway connected with Friends Service")
	return &Server{
		addr:             addr,
		websocketURL:     websocketURL,
		authServiceCli:   authService,
		friendServiceCli: friendsService,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	wsProxy, err := websocketproxy.NewHandler(s.websocketURL)
	if err != nil {
		return err
	}
	mux.Handle("GET /ws", auth.JWTMiddleware(wsProxy))

	authHandler := auth.NewHandler(auth.NewService(s.authServiceCli))
	mux.Handle("POST /auth/login", http.HandlerFunc(authHandler.LoginHandler))
	mux.Handle("POST /auth/register", http.HandlerFunc(authHandler.RegisterHandler))

	friendHandler := friends.NewHandler(friends.NewService(s.friendServiceCli))
	mux.Handle("POST /friends", auth.JWTMiddleware(http.HandlerFunc(friendHandler.Create)))
	mux.Handle("GET /friends", auth.JWTMiddleware(http.HandlerFunc(friendHandler.List)))
	mux.Handle("GET /friends/{id}", auth.JWTMiddleware(http.HandlerFunc(friendHandler.GetByID)))
	mux.Handle("PUT /friends/{id}", auth.JWTMiddleware(http.HandlerFunc(friendHandler.Update)))
	mux.Handle("DELETE /friends/{id}", auth.JWTMiddleware(http.HandlerFunc(friendHandler.Delete)))

	return http.ListenAndServe(s.addr, withCORS(mux))
}

func withCORS(next http.Handler) http.Handler {
	allowedOrigin := "http://localhost:5173"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
