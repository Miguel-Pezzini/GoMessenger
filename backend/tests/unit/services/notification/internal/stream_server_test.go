package notification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type streamRepositoryStub struct {
	presence    PresenceSnapshot
	presenceErr error
	published   []NotificationMessage
}

func (r *streamRepositoryStub) GetPresence(context.Context, string) (PresenceSnapshot, error) {
	return r.presence, r.presenceErr
}

func (r *streamRepositoryStub) PublishNotification(_ context.Context, _ string, notification NotificationMessage) error {
	r.published = append(r.published, notification)
	return nil
}

type fakeRedisServer struct {
	listener       net.Listener
	mu             sync.Mutex
	ackCount       int
	createCount    int
	readGroupError string
	shutdownOnce   sync.Once
	acceptedConns  sync.WaitGroup
}

func newFakeRedisServer(t *testing.T, readGroupError string) *fakeRedisServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	server := &fakeRedisServer{
		listener:       listener,
		readGroupError: readGroupError,
	}
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
			t.Logf("fake redis stopped after read error: %v", err)
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
		case "XGROUP":
			s.mu.Lock()
			s.createCount++
			s.mu.Unlock()
			if _, err := fmt.Fprint(conn, "+OK\r\n"); err != nil {
				return
			}
		case "XREADGROUP":
			if s.readGroupError != "" {
				if _, err := fmt.Fprintf(conn, "-NOGROUP %s\r\n", s.readGroupError); err != nil {
					return
				}
				s.readGroupError = ""
				continue
			}
			if _, err := fmt.Fprint(conn, "$-1\r\n"); err != nil {
				return
			}
		case "XAUTOCLAIM":
			if _, err := fmt.Fprint(conn, "*3\r\n*0\r\n$3\r\n0-0\r\n*0\r\n"); err != nil {
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

func newRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		MaxRetries:   0,
	})
}

func TestProcessFriendMessageAcknowledgesInvalidPayload(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	server := NewStreamServer(newRedisClient(fakeRedis.addr()), NewService(&streamRepositoryStub{}, "notifications"), "friend-stream", "friend-group", "message-stream", "message-group", "notification-consumer")
	defer server.rdb.Close()

	err := server.processFriendMessage(context.Background(), "friend-group", redis.XMessage{ID: "1-0", Values: map[string]any{}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.ackCount != 1 {
		t.Fatalf("expected one ack, got %d", fakeRedis.ackCount)
	}
}

func TestProcessMessageNotificationPublishesAndAcknowledges(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	repo := &streamRepositoryStub{presenceErr: ErrPresenceNotFound}
	server := NewStreamServer(newRedisClient(fakeRedis.addr()), NewService(repo, "notifications"), "friend-stream", "friend-group", "message-stream", "message-group", "notification-consumer")
	defer server.rdb.Close()

	err := server.processMessageNotification(context.Background(), "message-group", redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"type":"message_received","event_id":"msg-1","message_id":"msg-1","sender_id":"user-a","receiver_id":"user-b","content":"hello","occurred_at":"2026-04-08T00:00:00Z"}`,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.published) != 1 {
		t.Fatalf("expected one notification publish, got %d", len(repo.published))
	}
	if fakeRedis.ackCount != 1 {
		t.Fatalf("expected one ack, got %d", fakeRedis.ackCount)
	}
}

func TestProcessMessageNotificationLeavesPendingOnPresenceFailure(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	repo := &streamRepositoryStub{presenceErr: errors.New("redis unavailable")}
	server := NewStreamServer(newRedisClient(fakeRedis.addr()), NewService(repo, "notifications"), "friend-stream", "friend-group", "message-stream", "message-group", "notification-consumer")
	defer server.rdb.Close()

	err := server.processMessageNotification(context.Background(), "message-group", redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"type":"message_received","event_id":"msg-1","message_id":"msg-1","sender_id":"user-a","receiver_id":"user-b","content":"hello","occurred_at":"2026-04-08T00:00:00Z"}`,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.ackCount != 0 {
		t.Fatalf("expected message to remain pending, got %d acks", fakeRedis.ackCount)
	}
}

func TestProcessNewMessagesRecreatesConsumerGroupOnNoGroup(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "consumer group missing")
	defer fakeRedis.close()

	server := NewStreamServer(newRedisClient(fakeRedis.addr()), NewService(&streamRepositoryStub{}, "notifications"), "friend-stream", "friend-group", "message-stream", "message-group", "notification-consumer")
	defer server.rdb.Close()

	err := server.processNewMessages(context.Background(), "message-stream", "message-group", server.processMessageNotification)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.createCount == 0 {
		t.Fatal("expected consumer group recreation")
	}
}
