package media

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
)

type Config struct {
	Address          string
	MongoURI         string
	MongoDatabase    string
	StorageEndpoint  string
	StorageRegion    string
	StorageBucket    string
	StorageAccessKey string
	StorageSecretKey string
	InternalToken    string
	UploadTTL        time.Duration
	CleanupInterval  time.Duration
}

func LoadConfig() Config {
	return Config{
		Address:          config.MustString("MEDIA_ADDR"),
		MongoURI:         config.MustString("MEDIA_MONGO_URI"),
		MongoDatabase:    config.MustString("MEDIA_MONGO_DB"),
		StorageEndpoint:  config.MustString("MEDIA_STORAGE_ENDPOINT"),
		StorageRegion:    config.String("MEDIA_STORAGE_REGION", "us-east-1"),
		StorageBucket:    config.MustString("MEDIA_STORAGE_BUCKET"),
		StorageAccessKey: config.MustString("MEDIA_STORAGE_ACCESS_KEY"),
		StorageSecretKey: config.MustString("MEDIA_STORAGE_SECRET_KEY"),
		InternalToken:    config.String("INTERNAL_SERVICE_TOKEN", "dev-internal-token"),
		UploadTTL:        parseDuration(config.String("MEDIA_UPLOAD_TTL", "1h"), time.Hour),
		CleanupInterval:  parseDuration(config.String("MEDIA_CLEANUP_INTERVAL", "10m"), 10*time.Minute),
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

	storage := NewS3Storage(S3Config{
		Endpoint:  cfg.StorageEndpoint,
		Region:    cfg.StorageRegion,
		Bucket:    cfg.StorageBucket,
		AccessKey: cfg.StorageAccessKey,
		SecretKey: cfg.StorageSecretKey,
	})
	if err := ensureStorageReady(context.Background(), storage); err != nil {
		return err
	}

	service := NewService(repo, storage, cfg.UploadTTL)
	handler := NewHandler(service, cfg.InternalToken)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments", handler.UploadAttachments)
	mux.HandleFunc("GET /attachments/{id}", handler.DownloadAttachment)
	mux.HandleFunc("POST /internal/attachments/prepare-message", handler.PrepareMessage)
	mux.HandleFunc("POST /internal/attachments/bind-message", handler.BindMessage)
	mux.HandleFunc("POST /internal/attachments/cleanup-expired", handler.CleanupExpired)

	go startCleanupLoop(service, cfg.CleanupInterval)

	log.Printf("media service listening on %s", cfg.Address)
	return http.ListenAndServe(cfg.Address, mux)
}

func ensureStorageReady(ctx context.Context, storage Storage) error {
	var lastErr error
	for i := 0; i < 30; i++ {
		if err := storage.EnsureBucket(ctx); err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		return nil
	}
	return lastErr
}

func startCleanupLoop(service *Service, interval time.Duration) {
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if _, err := service.CleanupExpired(ctx, time.Now().UTC(), 100); err != nil {
			log.Printf("media cleanup failed: %v", err)
		}
		cancel()
	}
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}
