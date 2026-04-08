package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var loadEnvOnce sync.Once

func String(key, fallback string) string {
	loadEnv()

	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func MustString(key string) string {
	loadEnv()

	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("missing required environment variable %s", key))
	}
	return value
}

func loadEnv() {
	loadEnvOnce.Do(func() {
		path, err := findDotEnv()
		if err != nil || path == "" {
			return
		}
		_ = loadFile(path)
	})
}

func findDotEnv() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		if key == "" || os.Getenv(key) != "" {
			continue
		}

		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}
