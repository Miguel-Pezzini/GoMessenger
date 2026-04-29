package observability

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/Miguel-Pezzini/GoMessenger/internal/platform/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

const namespace = "gomessenger"

type Observer struct {
	serviceName string
	registry    *prometheus.Registry

	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpInFlight        *prometheus.GaugeVec

	websocketActive              prometheus.Gauge
	websocketConnectionsTotal    *prometheus.CounterVec
	websocketDisconnectionsTotal *prometheus.CounterVec
	websocketInboundTotal        *prometheus.CounterVec
	websocketMessageDuration     *prometheus.HistogramVec
	websocketOutboundTotal       *prometheus.CounterVec
	websocketOutboundDuration    *prometheus.HistogramVec

	chatStreamMessagesTotal    *prometheus.CounterVec
	chatStreamProcessing       *prometheus.HistogramVec
	chatStreamMessageAge       *prometheus.HistogramVec
	chatStreamAcksTotal        *prometheus.CounterVec
	chatRedisOperationDuration *prometheus.HistogramVec
	chatMongoOperationDuration *prometheus.HistogramVec
	chatStreamLag              *prometheus.GaugeVec
	chatStreamPending          *prometheus.GaugeVec
}

type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

func New(serviceName string) *Observer {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = "unknown"
	}

	registry := prometheus.NewRegistry()
	observer := &Observer{
		serviceName: serviceName,
		registry:    registry,
		httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total HTTP requests handled by service, route, method, and status.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"route", "method", "status"}),
		httpRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration by service, route, method, and status.",
			Buckets:   requestDurationBuckets(),
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"route", "method", "status"}),
		httpInFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_in_flight_requests",
			Help:      "Current in-flight HTTP requests by service, route, and method.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"route", "method"}),
		websocketActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "websocket_active_connections",
			Help:      "Current active websocket connections.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}),
		websocketConnectionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "websocket_connections_total",
			Help:      "Total websocket connection attempts by result.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"result"}),
		websocketDisconnectionsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "websocket_disconnections_total",
			Help:      "Total websocket disconnections by reason.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"reason"}),
		websocketInboundTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "websocket_inbound_messages_total",
			Help:      "Total inbound websocket messages by message type and result.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"message_type", "result"}),
		websocketMessageDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "websocket_message_duration_seconds",
			Help:      "Duration spent handling inbound websocket messages.",
			Buckets:   requestDurationBuckets(),
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"message_type", "result"}),
		websocketOutboundTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "websocket_outbound_writes_total",
			Help:      "Total websocket outbound write attempts by result.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"result"}),
		websocketOutboundDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "websocket_outbound_write_duration_seconds",
			Help:      "Duration spent writing outbound websocket messages.",
			Buckets:   requestDurationBuckets(),
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"result"}),
		chatStreamMessagesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "chat_stream_messages_total",
			Help:      "Total chat stream messages processed by result.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"result"}),
		chatStreamProcessing: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "chat_stream_processing_duration_seconds",
			Help:      "Duration spent processing chat stream messages.",
			Buckets:   requestDurationBuckets(),
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"result"}),
		chatStreamMessageAge: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "chat_stream_message_age_seconds",
			Help:      "Age of chat stream messages at processing time.",
			Buckets:   latencyBuckets(),
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"result"}),
		chatStreamAcksTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "chat_stream_acks_total",
			Help:      "Total chat stream ack attempts by result.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"result"}),
		chatRedisOperationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "chat_redis_operation_duration_seconds",
			Help:      "Duration of chat Redis operations.",
			Buckets:   requestDurationBuckets(),
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"operation", "result"}),
		chatMongoOperationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "chat_mongo_operation_duration_seconds",
			Help:      "Duration of chat MongoDB operations.",
			Buckets:   requestDurationBuckets(),
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"operation", "result"}),
		chatStreamLag: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "chat_stream_lag",
			Help:      "Redis stream consumer group lag for chat stream messages.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"stream", "group"}),
		chatStreamPending: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "chat_stream_pending",
			Help:      "Redis stream consumer group pending count for chat stream messages.",
			ConstLabels: prometheus.Labels{
				"service": serviceName,
			},
		}, []string{"stream", "group"}),
	}

	prometheus.WrapRegistererWith(prometheus.Labels{"service": serviceName}, registry).MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	registry.MustRegister(
		observer.httpRequestsTotal,
		observer.httpRequestDuration,
		observer.httpInFlight,
		observer.websocketActive,
		observer.websocketConnectionsTotal,
		observer.websocketDisconnectionsTotal,
		observer.websocketInboundTotal,
		observer.websocketMessageDuration,
		observer.websocketOutboundTotal,
		observer.websocketOutboundDuration,
		observer.chatStreamMessagesTotal,
		observer.chatStreamProcessing,
		observer.chatStreamMessageAge,
		observer.chatStreamAcksTotal,
		observer.chatRedisOperationDuration,
		observer.chatMongoOperationDuration,
		observer.chatStreamLag,
		observer.chatStreamPending,
	)

	return observer
}

func (o *Observer) Handler(next http.Handler) http.Handler {
	if o == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		method := r.Method
		inFlightRoute := routeLabel(r)

		o.httpInFlight.WithLabelValues(inFlightRoute, method).Inc()
		defer o.httpInFlight.WithLabelValues(inFlightRoute, method).Dec()

		next.ServeHTTP(recorder, r)

		route := routeLabel(r)
		status := strconv.Itoa(recorder.statusCode)
		o.httpRequestsTotal.WithLabelValues(route, method, status).Inc()
		o.httpRequestDuration.WithLabelValues(route, method, status).Observe(time.Since(start).Seconds())
	})
}

func (o *Observer) Mount(mux *http.ServeMux, checks ...ReadinessCheck) {
	if o == nil || mux == nil {
		return
	}

	mux.Handle("GET /metrics", promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.Handle("GET /readyz", readinessHandler(checks...))

	if pprofEnabled() {
		mountPprof(mux)
	}
}

func (o *Observer) Registry() *prometheus.Registry {
	if o == nil {
		return nil
	}
	return o.registry
}

func (o *Observer) WebSocketConnected() {
	if o == nil {
		return
	}
	o.websocketActive.Inc()
	o.websocketConnectionsTotal.WithLabelValues("success").Inc()
}

func (o *Observer) WebSocketConnectFailed() {
	if o == nil {
		return
	}
	o.websocketConnectionsTotal.WithLabelValues("failure").Inc()
}

func (o *Observer) WebSocketDisconnected(reason string) {
	if o == nil {
		return
	}
	reason = cleanLabel(reason, "unknown")
	o.websocketActive.Dec()
	o.websocketDisconnectionsTotal.WithLabelValues(reason).Inc()
}

func (o *Observer) ObserveWebSocketMessage(messageType, result string, duration time.Duration) {
	if o == nil {
		return
	}
	messageType = cleanLabel(messageType, "unknown")
	result = cleanLabel(result, "unknown")
	o.websocketInboundTotal.WithLabelValues(messageType, result).Inc()
	o.websocketMessageDuration.WithLabelValues(messageType, result).Observe(duration.Seconds())
}

func (o *Observer) ObserveWebSocketWrite(result string, duration time.Duration) {
	if o == nil {
		return
	}
	result = cleanLabel(result, "unknown")
	o.websocketOutboundTotal.WithLabelValues(result).Inc()
	o.websocketOutboundDuration.WithLabelValues(result).Observe(duration.Seconds())
}

func (o *Observer) ObserveChatStreamMessage(result string, age, duration time.Duration) {
	if o == nil {
		return
	}
	result = cleanLabel(result, "unknown")
	o.chatStreamMessagesTotal.WithLabelValues(result).Inc()
	o.chatStreamProcessing.WithLabelValues(result).Observe(duration.Seconds())
	if age >= 0 {
		o.chatStreamMessageAge.WithLabelValues(result).Observe(age.Seconds())
	}
}

func (o *Observer) ObserveChatStreamAck(result string) {
	if o == nil {
		return
	}
	o.chatStreamAcksTotal.WithLabelValues(cleanLabel(result, "unknown")).Inc()
}

func (o *Observer) ObserveChatRedisOperation(operation, result string, duration time.Duration) {
	if o == nil {
		return
	}
	o.chatRedisOperationDuration.WithLabelValues(cleanLabel(operation, "unknown"), cleanLabel(result, "unknown")).Observe(duration.Seconds())
}

func (o *Observer) ObserveChatMongoOperation(operation, result string, duration time.Duration) {
	if o == nil {
		return
	}
	o.chatMongoOperationDuration.WithLabelValues(cleanLabel(operation, "unknown"), cleanLabel(result, "unknown")).Observe(duration.Seconds())
}

func (o *Observer) SetChatStreamLag(streamName, groupName string, lag float64) {
	if o == nil {
		return
	}
	o.chatStreamLag.WithLabelValues(cleanLabel(streamName, "unknown"), cleanLabel(groupName, "unknown")).Set(lag)
}

func (o *Observer) SetChatStreamPending(streamName, groupName string, pending float64) {
	if o == nil {
		return
	}
	o.chatStreamPending.WithLabelValues(cleanLabel(streamName, "unknown"), cleanLabel(groupName, "unknown")).Set(pending)
}

func RedisPingCheck(name string, client *redis.Client) ReadinessCheck {
	return ReadinessCheck{
		Name: name,
		Check: func(ctx context.Context) error {
			if client == nil {
				return errors.New("redis client is nil")
			}
			return client.Ping(ctx).Err()
		},
	}
}

func MongoPingCheck(name string, db *mongo.Database) ReadinessCheck {
	return ReadinessCheck{
		Name: name,
		Check: func(ctx context.Context) error {
			if db == nil {
				return errors.New("mongo database is nil")
			}
			return db.Client().Ping(ctx, nil)
		},
	}
}

func HTTPHealthCheck(name, baseURL string) ReadinessCheck {
	return ReadinessCheck{
		Name: name,
		Check: func(ctx context.Context) error {
			target := strings.TrimRight(baseURL, "/") + "/healthz"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				return errors.New(resp.Status)
			}
			return nil
		},
	}
}

func FunctionCheck(name string, check func(context.Context) error) ReadinessCheck {
	return ReadinessCheck{Name: name, Check: check}
}

func StreamMessageAge(streamID string, now time.Time) time.Duration {
	millisPart, _, ok := strings.Cut(streamID, "-")
	if !ok {
		return -1
	}
	millis, err := strconv.ParseInt(millisPart, 10, 64)
	if err != nil {
		return -1
	}
	createdAt := time.UnixMilli(millis)
	if createdAt.After(now) {
		return 0
	}
	return now.Sub(createdAt)
}

func requestDurationBuckets() []float64 {
	return []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
}

func latencyBuckets() []float64 {
	return []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}
}

func routeLabel(r *http.Request) string {
	if r != nil && r.Pattern != "" {
		return r.Pattern
	}
	return "unknown"
}

func cleanLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readinessHandler(checks ...ReadinessCheck) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		results := make(map[string]string, len(checks))
		status := http.StatusOK
		for _, check := range checks {
			if check.Name == "" || check.Check == nil {
				continue
			}
			if err := check.Check(ctx); err != nil {
				results[check.Name] = err.Error()
				status = http.StatusServiceUnavailable
				continue
			}
			results[check.Name] = "ok"
		}

		responseStatus := "ready"
		if status != http.StatusOK {
			responseStatus = "not_ready"
		}
		writeJSON(w, status, map[string]any{
			"status": responseStatus,
			"checks": results,
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func pprofEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(config.String("OBSERVABILITY_PPROF_ENABLED", "false"))) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func mountPprof(mux *http.ServeMux) {
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(data)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (r *statusRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if readerFrom, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		if !r.wroteHeader {
			r.WriteHeader(http.StatusOK)
		}
		return readerFrom.ReadFrom(reader)
	}
	return io.Copy(r.ResponseWriter, reader)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
