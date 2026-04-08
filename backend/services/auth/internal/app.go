package auth

import (
	"log"
	"net/http"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
)

type Config struct {
	Address       string
	MongoURI      string
	MongoDatabase string
	RedisAddr     string
	AuditStream   string
	JWTSecret     string
	JWTExpiry     time.Duration
}

func LoadConfig() Config {
	return Config{
		Address:       config.MustString("AUTH_ADDR"),
		MongoURI:      config.MustString("AUTH_MONGO_URI"),
		MongoDatabase: config.MustString("AUTH_MONGO_DB"),
		RedisAddr:     config.MustString("REDIS_ADDR"),
		AuditStream:   config.MustString("REDIS_STREAM_AUDIT_LOGS"),
		JWTSecret:     config.MustString("JWT_SECRET"),
		JWTExpiry:     parseJWTExpiry(config.String("JWT_EXPIRY", "")),
	}
}

func parseJWTExpiry(raw string) time.Duration {
	if raw == "" {
		return 24 * time.Hour
	}

	expiry, err := time.ParseDuration(raw)
	if err != nil || expiry <= 0 {
		panic("invalid JWT_EXPIRY: must be a positive duration")
	}

	return expiry
}

func Run() error {
	cfg := LoadConfig()

	db, err := mongoutil.NewDatabase(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return err
	}
	rdb, err := redisutil.NewClient(cfg.RedisAddr)
	if err != nil {
		return err
	}

	service := NewService(
		NewMongoRepository(db),
		NewTokenIssuer(cfg.JWTSecret, cfg.JWTExpiry),
	)

	handler := NewHandler(service, audit.NewRedisPublisher(rdb, cfg.AuditStream))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", handler.Register)
	mux.HandleFunc("POST /auth/login", handler.Login)

	log.Printf("auth service listening on %s", cfg.Address)
	defer rdb.Close()
	return http.ListenAndServe(cfg.Address, mux)
}
