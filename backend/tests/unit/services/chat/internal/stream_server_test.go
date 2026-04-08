package chat

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

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/audit"
	"github.com/redis/go-redis/v9"
)

func TestDecodeMessage(t *testing.T) {
	msg := redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"sender_id":"user-a","receiver_id":"user-b","content":"hello","timestamp":123}`,
		},
	}

	req, err := decodeMessage(msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if req.SenderID != "user-a" {
		t.Fatalf("expected sender user-a, got %s", req.SenderID)
	}
	if req.ReceiverID != "user-b" {
		t.Fatalf("expected receiver user-b, got %s", req.ReceiverID)
	}
}

func TestDecodeMessageRejectsMissingPayload(t *testing.T) {
	_, err := decodeMessage(redis.XMessage{
		ID:     "1-0",
		Values: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

type streamRepositoryStub struct {
	result           *MessageDB
	err              error
	updateMessageID  string
	updateReceiverID string
	updateStatus     string
	updateResult     *MessageDB
	updateErr        error
}

type auditPublisherStub struct {
	events []audit.Event
}

func (r *streamRepositoryStub) Create(_ context.Context, _ *MessageDB) (*MessageDB, bool, error) {
	if r.err != nil {
		return nil, false, r.err
	}
	return r.result, true, nil
}

func (r *streamRepositoryStub) GetConversation(_ context.Context, _, _, _ string, _ int) ([]MessageDB, error) {
	return nil, nil
}

func (r *streamRepositoryStub) UpdateViewedStatus(_ context.Context, messageID, receiverUserID, status string) (*MessageDB, error) {
	r.updateMessageID = messageID
	r.updateReceiverID = receiverUserID
	r.updateStatus = status
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	return r.updateResult, nil
}

func (p *auditPublisherStub) Publish(_ context.Context, event audit.Event) error {
	p.events = append(p.events, event.Normalize())
	return nil
}

type fakeRedisServer struct {
	listener      net.Listener
	mu            sync.Mutex
	ackCount      int
	publishCount  int
	publishErr    string
	readTimeout   time.Duration
	shutdownOnce  sync.Once
	acceptedConns sync.WaitGroup
}

func newFakeRedisServer(t *testing.T, publishErr string) *fakeRedisServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake redis server: %v", err)
	}

	server := &fakeRedisServer{
		listener:    listener,
		publishErr:  publishErr,
		readTimeout: 2 * time.Second,
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
		_ = conn.SetReadDeadline(time.Now().Add(s.readTimeout))
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
		case "PUBLISH":
			s.mu.Lock()
			s.publishCount++
			s.mu.Unlock()

			if s.publishErr != "" {
				if _, err := fmt.Fprintf(conn, "-ERR %s\r\n", s.publishErr); err != nil {
					return
				}
				continue
			}
			if _, err := fmt.Fprint(conn, ":1\r\n"); err != nil {
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

func newRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		MaxRetries:   0,
	})
}

func TestProcessMessageAcknowledgesInvalidPayload(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	server := NewServer(
		"chat-consumer",
		"chat-stream",
		"chat-channel",
		"chat-events",
		"notification-stream",
		newRedisClient(fakeRedis.addr()),
		NewService(&streamRepositoryStub{}),
		&auditPublisherStub{},
	)
	defer server.rdb.Close()

	err := server.processMessage(context.Background(), redis.XMessage{
		ID:     "1-0",
		Values: map[string]any{},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.ackCount != 1 {
		t.Fatalf("expected invalid message to be acked once, got %d", fakeRedis.ackCount)
	}
	if fakeRedis.publishCount != 0 {
		t.Fatalf("expected invalid message not to be published, got %d publishes", fakeRedis.publishCount)
	}
}

func TestProcessMessageLeavesMessagePendingWhenMongoWriteFails(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	server := NewServer(
		"chat-consumer",
		"chat-stream",
		"chat-channel",
		"chat-events",
		"notification-stream",
		newRedisClient(fakeRedis.addr()),
		NewService(&streamRepositoryStub{err: errors.New("mongo unavailable")}),
		&auditPublisherStub{},
	)
	defer server.rdb.Close()

	err := server.processMessage(context.Background(), redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"sender_id":"user-a","receiver_id":"user-b","content":"hello","timestamp":123}`,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.ackCount != 0 {
		t.Fatalf("expected failed persistence to leave message pending, got %d acks", fakeRedis.ackCount)
	}
	if fakeRedis.publishCount != 0 {
		t.Fatalf("expected failed persistence not to publish, got %d publishes", fakeRedis.publishCount)
	}
}

func TestProcessMessageLeavesMessagePendingWhenPublishFails(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "publish failed")
	defer fakeRedis.close()

	server := NewServer(
		"chat-consumer",
		"chat-stream",
		"chat-channel",
		"chat-events",
		"notification-stream",
		newRedisClient(fakeRedis.addr()),
		NewService(&streamRepositoryStub{result: &MessageDB{
			Id:           "mongo-id",
			StreamID:     "1-0",
			SenderID:     "user-a",
			ReceiverID:   "user-b",
			Content:      "hello",
			Timestamp:    123,
			ViewedStatus: ViewedStatusSent,
		}}),
		&auditPublisherStub{},
	)
	defer server.rdb.Close()

	err := server.processMessage(context.Background(), redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"sender_id":"user-a","receiver_id":"user-b","content":"hello","timestamp":123}`,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.publishCount != 1 {
		t.Fatalf("expected one publish attempt, got %d", fakeRedis.publishCount)
	}
	if fakeRedis.ackCount != 0 {
		t.Fatalf("expected publish failure to leave message pending, got %d acks", fakeRedis.ackCount)
	}
}

func TestProcessMessagePublishesAndAcknowledgesPersistedMessages(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	server := NewServer(
		"chat-consumer",
		"chat-stream",
		"chat-channel",
		"chat-events",
		"notification-stream",
		newRedisClient(fakeRedis.addr()),
		NewService(&streamRepositoryStub{result: &MessageDB{
			Id:           "mongo-id",
			StreamID:     "1-0",
			SenderID:     "user-a",
			ReceiverID:   "user-b",
			Content:      "hello",
			Timestamp:    123,
			ViewedStatus: ViewedStatusSent,
		}}),
		&auditPublisherStub{},
	)
	defer server.rdb.Close()

	err := server.processMessage(context.Background(), redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"sender_id":"user-a","receiver_id":"user-b","content":"hello","timestamp":123}`,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeRedis.publishCount != 1 {
		t.Fatalf("expected one publish, got %d", fakeRedis.publishCount)
	}
	if fakeRedis.ackCount != 1 {
		t.Fatalf("expected one ack, got %d", fakeRedis.ackCount)
	}
}

func TestProcessMessagePublishesAuditEventOnSuccess(t *testing.T) {
	fakeRedis := newFakeRedisServer(t, "")
	defer fakeRedis.close()

	auditPublisher := &auditPublisherStub{}
	server := NewServer(
		"chat-consumer",
		"chat-stream",
		"chat-channel",
		"chat-events",
		"notification-stream",
		newRedisClient(fakeRedis.addr()),
		NewService(&streamRepositoryStub{result: &MessageDB{
			Id:           "mongo-id",
			StreamID:     "1-0",
			SenderID:     "user-a",
			ReceiverID:   "user-b",
			Content:      "hello",
			Timestamp:    123,
			ViewedStatus: ViewedStatusSent,
		}}),
		auditPublisher,
	)
	defer server.rdb.Close()

	_ = server.processMessage(context.Background(), redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"payload": `{"sender_id":"user-a","receiver_id":"user-b","content":"hello","timestamp":123}`,
		},
	})

	if len(auditPublisher.events) == 0 {
		t.Fatal("expected audit event to be published")
	}
	if auditPublisher.events[0].EventType != "chat.message.persisted" {
		t.Fatalf("expected chat.message.persisted, got %s", auditPublisher.events[0].EventType)
	}
}

func TestProcessInteractionEventUpdatesViewedStatus(t *testing.T) {
	repo := &streamRepositoryStub{
		updateResult: &MessageDB{
			Id:           "mongo-id",
			SenderID:     "user-a",
			ReceiverID:   "user-b",
			Content:      "hello",
			ViewedStatus: ViewedStatusSeen,
		},
	}
	server := NewServer(
		"chat-consumer",
		"chat-stream",
		"chat-channel",
		"chat-events",
		"",
		nil,
		NewService(repo),
		&auditPublisherStub{},
	)

	err := server.processInteractionEvent(context.Background(), InteractionEvent{
		Type:         "message_seen",
		ActorUserID:  "user-b",
		TargetUserID: "user-a",
		MessageID:    "mongo-id",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.updateMessageID != "mongo-id" {
		t.Fatalf("expected message id mongo-id, got %s", repo.updateMessageID)
	}
	if repo.updateReceiverID != "user-b" {
		t.Fatalf("expected receiver user-b, got %s", repo.updateReceiverID)
	}
	if repo.updateStatus != ViewedStatusSeen {
		t.Fatalf("expected status %s, got %s", ViewedStatusSeen, repo.updateStatus)
	}
}

func TestProcessInteractionEventIgnoresTypingEvents(t *testing.T) {
	repo := &streamRepositoryStub{}
	server := NewServer(
		"chat-consumer",
		"chat-stream",
		"chat-channel",
		"chat-events",
		"",
		nil,
		NewService(repo),
		&auditPublisherStub{},
	)

	if err := server.processInteractionEvent(context.Background(), InteractionEvent{
		Type:         "typing_started",
		ActorUserID:  "user-b",
		TargetUserID: "user-a",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.updateMessageID != "" {
		t.Fatalf("expected typing event to be ignored, got update for %s", repo.updateMessageID)
	}
}
