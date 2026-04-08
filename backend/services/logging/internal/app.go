package logging

import (
	"context"
	"net/http"
	"strings"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
	redisutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/redis"
	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/security"
)

type Config struct {
	Address          string
	MongoURI         string
	MongoDatabase    string
	RedisAddr        string
	RedisStreamAudit string
	AllowedOrigins   []string
}

func LoadConfig() Config {
	return Config{
		Address:          config.MustString("LOGGING_ADDR"),
		MongoURI:         config.MustString("LOGGING_MONGO_URI"),
		MongoDatabase:    config.MustString("LOGGING_MONGO_DB"),
		RedisAddr:        config.MustString("REDIS_ADDR"),
		RedisStreamAudit: config.MustString("REDIS_STREAM_AUDIT_LOGS"),
		AllowedOrigins:   parseAllowedOrigins(config.String("LOGGING_ALLOWED_ORIGINS", config.String("GATEWAY_ALLOWED_ORIGIN", ""))),
	}
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

	repo, err := NewMongoRepository(db)
	if err != nil {
		return err
	}

	service := NewService(repo)
	streamServer := NewServer(cfg.RedisStreamAudit, rdb, service)
	go streamServer.Start(context.Background())

	handler := NewHandler(service, security.NewOriginValidator(cfg.AllowedOrigins))
	mux := http.NewServeMux()
	mux.Handle("GET /logs", http.HandlerFunc(handler.ListLogs))
	mux.Handle("GET /logs/ws", http.HandlerFunc(handler.StreamLogs))

	return http.ListenAndServe(cfg.Address, mux)
}

func parseAllowedOrigins(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		origins = append(origins, part)
	}

	return origins
}
