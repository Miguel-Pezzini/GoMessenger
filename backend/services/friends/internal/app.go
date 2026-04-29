package friends

import (
	"log"
	"net/http"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/observability"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
)

type Config struct {
	Address                          string
	MongoURI                         string
	MongoDatabase                    string
	RedisAddr                        string
	FriendEvents                     string
	NotificationFriendRequestsStream string
	AuditStream                      string
	AuthUpstreamURL                  string
	InternalServiceToken             string
}

func LoadConfig() Config {
	return Config{
		Address:                          config.MustString("FRIENDS_ADDR"),
		MongoURI:                         config.MustString("FRIENDS_MONGO_URI"),
		MongoDatabase:                    config.MustString("FRIENDS_MONGO_DB"),
		RedisAddr:                        config.MustString("REDIS_ADDR"),
		FriendEvents:                     config.MustString("REDIS_CHANNEL_FRIEND_EVENTS"),
		NotificationFriendRequestsStream: config.MustString("REDIS_STREAM_NOTIFICATION_FRIEND_REQUESTS"),
		AuditStream:                      config.MustString("REDIS_STREAM_AUDIT_LOGS"),
		AuthUpstreamURL:                  config.MustString("AUTH_UPSTREAM_URL"),
		InternalServiceToken:             config.String("INTERNAL_SERVICE_TOKEN", "dev-internal-token"),
	}
}

func Run() error {
	cfg := LoadConfig()

	db, err := mongoutil.NewDatabase(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return err
	}

	repo, err := NewMongoRepository(db)
	if err != nil {
		return err
	}

	rdb, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return err
	}

	authLookup := NewAuthHTTPClient(cfg.AuthUpstreamURL, cfg.InternalServiceToken)
	service := NewService(repo, authLookup)
	handler := NewHandler(
		service,
		NewPublisher(rdb, cfg.FriendEvents),
		NewPublisher(rdb, cfg.NotificationFriendRequestsStream),
		audit.NewRedisPublisher(rdb, cfg.AuditStream),
		cfg.FriendEvents,
		authLookup,
	)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /friends/requests", handler.SendFriendRequest)
	mux.HandleFunc("POST /friends/requests/{id}/accept", handler.AcceptFriendRequest)
	mux.HandleFunc("DELETE /friends/requests/{id}/decline", handler.DeclineFriendRequest)
	mux.HandleFunc("GET /friends/requests/pending", handler.ListPendingFriendRequests)
	mux.HandleFunc("GET /friends", handler.ListFriends)
	mux.HandleFunc("DELETE /friends/{friendId}", handler.RemoveFriend)

	observer := observability.New("friends")
	observer.Mount(
		mux,
		observability.MongoPingCheck("mongo", db),
		observability.RedisPingCheck("redis", rdb),
		observability.HTTPHealthCheck("auth", cfg.AuthUpstreamURL),
	)

	log.Printf("friends service listening on %s", cfg.Address)
	defer rdb.Close()
	return http.ListenAndServe(cfg.Address, observer.Handler(mux))
}
