package config

import "os"

// ConsumerName returns a unique Redis stream consumer name for this replica.
// Priority: POD_NAME (K8s) > HOSTNAME (Docker/Compose) > fallback.
func ConsumerName(fallback string) string {
	if v := os.Getenv("POD_NAME"); v != "" {
		return v
	}
	if v := os.Getenv("HOSTNAME"); v != "" {
		return v
	}
	return fallback
}
