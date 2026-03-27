package app

import (
	"log"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/domain/auth"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/domain/friends"
	authclient "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/infra/grpc/authclient"
	friendsclient "github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/infra/grpc/friendsclient"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/authhandler"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/friendshandler"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/middleware"
	"github.com/Miguel-Pezzini/GoMessenger/services/gateway/internal/transport/http/websocketproxy"
)

type Config struct {
	Address        string
	AuthAddress    string
	FriendsAddress string
	WebsocketURL   string
	AllowedOrigin  string
	JWTSecret      string
}

func LoadConfig() Config {
	return Config{
		Address:        config.MustString("GATEWAY_ADDR"),
		AuthAddress:    config.MustString("AUTH_GRPC_ADDR"),
		FriendsAddress: config.MustString("FRIENDS_GRPC_ADDR"),
		WebsocketURL:   config.MustString("WEBSOCKET_UPSTREAM_URL"),
		AllowedOrigin:  config.MustString("GATEWAY_ALLOWED_ORIGIN"),
		JWTSecret:      config.MustString("JWT_SECRET"),
	}
}

func Run() error {
	cfg := LoadConfig()

	authService, err := authclient.New(cfg.AuthAddress)
	if err != nil {
		return err
	}
	friendsService, err := friendsclient.New(cfg.FriendsAddress)
	if err != nil {
		return err
	}

	router, err := NewRouter(cfg, authService, friendsService)
	if err != nil {
		return err
	}

	log.Printf("gateway listening on %s", cfg.Address)
	return http.ListenAndServe(cfg.Address, router)
}

func NewRouter(cfg Config, authService auth.ServiceClient, friendsService friends.ServiceClient) (http.Handler, error) {
	mux := http.NewServeMux()

	jwtMiddleware := middleware.New(cfg.JWTSecret)
	wsProxy, err := websocketproxy.NewHandler(cfg.WebsocketURL)
	if err != nil {
		return nil, err
	}

	mux.Handle("GET /ws", jwtMiddleware.Wrap(wsProxy))

	authHTTP := authhandler.New(auth.NewService(authService))
	mux.Handle("POST /auth/login", http.HandlerFunc(authHTTP.LoginHandler))
	mux.Handle("POST /auth/register", http.HandlerFunc(authHTTP.RegisterHandler))

	friendsHTTP := friendshandler.New(friends.NewService(friendsService))
	mux.Handle("POST /friends/requests", jwtMiddleware.Wrap(http.HandlerFunc(friendsHTTP.SendFriendRequest)))
	mux.Handle("POST /friends/requests/{id}/accept", jwtMiddleware.Wrap(http.HandlerFunc(friendsHTTP.AcceptFriendRequest)))
	mux.Handle("DELETE /friends/requests/{id}/decline", jwtMiddleware.Wrap(http.HandlerFunc(friendsHTTP.DeclineFriendRequest)))
	mux.Handle("GET /friends/requests/pending", jwtMiddleware.Wrap(http.HandlerFunc(friendsHTTP.ListPendingFriendRequests)))
	mux.Handle("GET /friends", jwtMiddleware.Wrap(http.HandlerFunc(friendsHTTP.ListFriends)))
	mux.Handle("DELETE /friends/{friendId}", jwtMiddleware.Wrap(http.HandlerFunc(friendsHTTP.RemoveFriend)))

	return withCORS(cfg.AllowedOrigin, mux), nil
}

func withCORS(allowedOrigin string, next http.Handler) http.Handler {
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
