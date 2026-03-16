package app

import (
	"log"
	"net"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	mongoutil "github.com/Miguel-Pezzini/GoMessenger/internal/platform/mongo"
	friendspb "github.com/Miguel-Pezzini/GoMessenger/pkg/contracts/friendspb"
	"github.com/Miguel-Pezzini/GoMessenger/services/friends/internal/domain"
	mongorepo "github.com/Miguel-Pezzini/GoMessenger/services/friends/internal/infra/mongo"
	grpctransport "github.com/Miguel-Pezzini/GoMessenger/services/friends/internal/transport/grpc"
	"google.golang.org/grpc"
)

type Config struct {
	Address       string
	MongoURI      string
	MongoDatabase string
}

func LoadConfig() Config {
	return Config{
		Address:       config.String("FRIENDS_ADDR", ":50052"),
		MongoURI:      config.String("FRIENDS_MONGO_URI", "mongodb://localhost:27020"),
		MongoDatabase: config.String("FRIENDS_MONGO_DB", "friends_db"),
	}
}

func Run() error {
	cfg := LoadConfig()

	db, err := mongoutil.NewDatabase(cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return err
	}

	service := domain.NewService(mongorepo.NewRepository(db))
	server := grpctransport.NewServer(service)

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	friendspb.RegisterFriendsServiceServer(grpcServer, server)

	log.Printf("friends service listening on %s", cfg.Address)
	return grpcServer.Serve(listener)
}
