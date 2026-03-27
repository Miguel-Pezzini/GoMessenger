package app

import (
	"log"
	"net"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
	authpb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/authpb"
	"github.com/Miguel-Pezzini/GoMessenger/services/auth/internal/domain"
	mongorepo "github.com/Miguel-Pezzini/GoMessenger/services/auth/internal/infra/mongo"
	grpctransport "github.com/Miguel-Pezzini/GoMessenger/services/auth/internal/transport/grpc"
	"google.golang.org/grpc"
)

type Config struct {
	Address       string
	MongoURI      string
	MongoDatabase string
	JWTSecret     string
	JWTExpiry     time.Duration
}

func LoadConfig() Config {
	return Config{
		Address:       config.MustString("AUTH_ADDR"),
		MongoURI:      config.MustString("AUTH_MONGO_URI"),
		MongoDatabase: config.MustString("AUTH_MONGO_DB"),
		JWTSecret:     config.MustString("JWT_SECRET"),
		JWTExpiry:     24 * time.Hour,
	}
}

func Run() error {
	cfg := LoadConfig()

	db, err := mongoutil.NewDatabase(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return err
	}

	service := domain.NewService(
		mongorepo.NewRepository(db),
		domain.NewTokenIssuer(cfg.JWTSecret, cfg.JWTExpiry),
	)

	server := grpctransport.NewServer(service)
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServiceServer(grpcServer, server)

	log.Printf("auth service listening on %s", cfg.Address)
	return grpcServer.Serve(listener)
}
