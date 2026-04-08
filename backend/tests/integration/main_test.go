package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMain(m *testing.M) {
	if err := resetTestState(); err != nil {
		log.Printf("warning: could not reset test state: %v", err)
	}
	os.Exit(m.Run())
}

// resetTestState drops all collections in the test databases and flushes Redis.
// Failures are non-fatal — tests will skip themselves if services are unreachable.
func resetTestState() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	authURI := envOrDefault("AUTH_MONGO_URI", "mongodb://localhost:27029")
	authDB := envOrDefault("AUTH_MONGO_DB", "userdb")

	chatURI := envOrDefault("CHAT_MONGO_URI", "mongodb://localhost:27028")
	chatDB := envOrDefault("CHAT_MONGO_DB", "chatdb")

	friendsURI := envOrDefault("FRIENDS_MONGO_URI", "mongodb://localhost:27030")
	friendsDB := envOrDefault("FRIENDS_MONGO_DB", "friends_db")
	loggingURI := envOrDefault("LOGGING_MONGO_URI", "mongodb://localhost:27031")
	loggingDB := envOrDefault("LOGGING_MONGO_DB", "logging_db")

	redisAddr := envOrDefault("REDIS_ADDR", "localhost:6380")

	var errs []error

	if err := dropMongoDatabase(ctx, authURI, authDB); err != nil {
		errs = append(errs, fmt.Errorf("auth db: %w", err))
	}
	if err := dropMongoDatabase(ctx, chatURI, chatDB); err != nil {
		errs = append(errs, fmt.Errorf("chat db: %w", err))
	}
	if err := dropMongoDatabase(ctx, friendsURI, friendsDB); err != nil {
		errs = append(errs, fmt.Errorf("friends db: %w", err))
	}
	if err := dropMongoDatabase(ctx, loggingURI, loggingDB); err != nil {
		errs = append(errs, fmt.Errorf("logging db: %w", err))
	}
	if err := flushRedis(redisAddr); err != nil {
		errs = append(errs, fmt.Errorf("redis: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("reset errors: %v", errs)
	}
	return nil
}

func dropMongoDatabase(ctx context.Context, uri, dbName string) error {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(normalizeLocalMongoURI(uri)))
	if err != nil {
		return err
	}
	defer client.Disconnect(ctx)
	return client.Database(dbName).Drop(ctx)
}

func flushRedis(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = fmt.Fprintf(conn, "*1\r\n$8\r\nFLUSHALL\r\n")
	return err
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func normalizeLocalMongoURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil {
		return uri
	}

	if parsed.Scheme != "mongodb" && parsed.Scheme != "mongodb+srv" {
		return uri
	}

	query := parsed.Query()
	if query.Has("directConnection") {
		return uri
	}

	host := parsed.Host
	if strings.Contains(host, ",") {
		return uri
	}

	if strings.Contains(host, "@") {
		parts := strings.SplitN(host, "@", 2)
		host = parts[1]
	}

	hostName := host
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		hostName = splitHost
	}

	switch hostName {
	case "localhost", "127.0.0.1":
		query.Set("directConnection", "true")
		if parsed.Path == "" {
			parsed.Path = "/"
		}
		parsed.RawQuery = query.Encode()
		return parsed.String()
	default:
		return uri
	}
}
