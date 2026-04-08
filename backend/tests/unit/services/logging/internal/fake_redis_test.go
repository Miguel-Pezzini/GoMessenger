package logging

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisServer struct {
	listener      net.Listener
	mu            sync.Mutex
	ackCount      int
	shutdownOnce  sync.Once
	acceptedConns sync.WaitGroup
}

func newFakeRedisServer(t *testing.T, _ string) *fakeRedisServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake redis server: %v", err)
	}

	server := &fakeRedisServer{listener: listener}
	server.acceptedConns.Add(1)
	go server.serve(t)
	return server
}

func (s *fakeRedisServer) serve(t *testing.T) {
	defer s.acceptedConns.Done()

	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		command, err := readRESPArray(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return
			}
			if strings.Contains(err.Error(), "closed") {
				return
			}
			t.Logf("fake redis server stopped after read error: %v", err)
			return
		}

		if len(command) == 0 {
			if _, err := fmt.Fprint(conn, "-ERR empty command\r\n"); err != nil {
				return
			}
			continue
		}

		switch strings.ToUpper(command[0]) {
		case "HELLO":
			if _, err := fmt.Fprint(conn, "%2\r\n+server\r\n+redis\r\n+version\r\n+7.0.0\r\n"); err != nil {
				return
			}
		case "CLIENT":
			if _, err := fmt.Fprint(conn, "+OK\r\n"); err != nil {
				return
			}
		case "XACK":
			s.mu.Lock()
			s.ackCount++
			s.mu.Unlock()
			if _, err := fmt.Fprint(conn, ":1\r\n"); err != nil {
				return
			}
		default:
			if _, err := fmt.Fprint(conn, "+OK\r\n"); err != nil {
				return
			}
		}
	}
}

func (s *fakeRedisServer) close() {
	s.shutdownOnce.Do(func() {
		_ = s.listener.Close()
		s.acceptedConns.Wait()
	})
}

func (s *fakeRedisServer) addr() string {
	return s.listener.Addr().String()
}

func newRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		MaxRetries:   0,
	})
}

func readRESPArray(conn net.Conn) ([]string, error) {
	line, err := readLine(conn)
	if err != nil {
		return nil, err
	}
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("unexpected frame %q", line)
	}

	var count int
	if _, err := fmt.Sscanf(line, "*%d", &count); err != nil {
		return nil, err
	}

	values := make([]string, 0, count)
	for i := 0; i < count; i++ {
		bulkHeader, err := readLine(conn)
		if err != nil {
			return nil, err
		}
		if len(bulkHeader) == 0 || bulkHeader[0] != '$' {
			return nil, fmt.Errorf("unexpected bulk header %q", bulkHeader)
		}

		var size int
		if _, err := fmt.Sscanf(bulkHeader, "$%d", &size); err != nil {
			return nil, err
		}

		payload := make([]byte, size+2)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return nil, err
		}
		values = append(values, string(payload[:size]))
	}

	return values, nil
}

func readLine(conn net.Conn) (string, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			return "", err
		}
		line = append(line, buf[0])
		if len(line) >= 2 && line[len(line)-2] == '\r' && line[len(line)-1] == '\n' {
			return string(line[:len(line)-2]), nil
		}
	}
}
