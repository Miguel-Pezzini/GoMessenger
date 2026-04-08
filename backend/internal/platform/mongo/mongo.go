package mongo

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	gomongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func NewDatabase(uri, dbName string) (*gomongo.Database, error) {
	client, err := gomongo.NewClient(options.Client().ApplyURI(normalizeLocalURI(uri)))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client.Database(dbName), nil
}

func normalizeLocalURI(uri string) string {
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

	hosts := strings.Split(parsed.Host, ",")
	if len(hosts) != 1 {
		return uri
	}

	host := hosts[0]
	if strings.Contains(host, "@") {
		parts := strings.SplitN(host, "@", 2)
		host = parts[1]
	}

	hostName, _, err := net.SplitHostPort(host)
	if err != nil {
		hostName = host
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
