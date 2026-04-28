package media

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

type S3Storage struct {
	cfg    S3Config
	client *http.Client
	now    func() time.Time
}

func NewS3Storage(cfg S3Config) *S3Storage {
	return &S3Storage{
		cfg:    cfg,
		client: http.DefaultClient,
		now:    time.Now,
	}
}

func (s *S3Storage) EnsureBucket(ctx context.Context) error {
	resp, err := s.do(ctx, http.MethodPut, "", "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict {
		return nil
	}
	return s.responseError(resp, "ensure bucket")
}

func (s *S3Storage) PutObject(ctx context.Context, key, contentType string, size int64, body interface {
	Read([]byte) (int, error)
}) error {
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(payload)) != size {
		return fmt.Errorf("object size mismatch: expected %d bytes, read %d", size, len(payload))
	}

	resp, err := s.do(ctx, http.MethodPut, key, contentType, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return s.responseError(resp, "put object")
}

func (s *S3Storage) GetObject(ctx context.Context, key string) (*StoredObject, error) {
	resp, err := s.do(ctx, http.MethodGet, key, "", nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, s.responseError(resp, "get object")
	}

	return &StoredObject{
		Body:          resp.Body,
		ContentLength: resp.ContentLength,
		ContentType:   resp.Header.Get("Content-Type"),
	}, nil
}

func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	resp, err := s.do(ctx, http.MethodDelete, key, "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil
	}
	return s.responseError(resp, "delete object")
}

func (s *S3Storage) do(ctx context.Context, method, key, contentType string, payload []byte) (*http.Response, error) {
	if strings.TrimSpace(s.cfg.Endpoint) == "" {
		return nil, fmt.Errorf("media storage endpoint is required")
	}
	if strings.TrimSpace(s.cfg.Bucket) == "" {
		return nil, fmt.Errorf("media storage bucket is required")
	}
	if strings.TrimSpace(s.cfg.Region) == "" {
		s.cfg.Region = "us-east-1"
	}

	target, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}

	payloadHash := sha256Hex(payload)
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(payload))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("X-Amz-Date", s.now().UTC().Format("20060102T150405Z"))

	s.sign(req, payloadHash)
	return s.client.Do(req)
}

func (s *S3Storage) objectURL(key string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimRight(s.cfg.Endpoint, "/"))
	if err != nil {
		return nil, err
	}

	path := strings.TrimRight(target.Path, "/") + "/" + s.cfg.Bucket
	if key != "" {
		path += "/" + key
	}
	target.Path = path
	return target, nil
}

func (s *S3Storage) sign(req *http.Request, payloadHash string) {
	requestTime, _ := time.Parse("20060102T150405Z", req.Header.Get("X-Amz-Date"))
	date := requestTime.UTC().Format("20060102")
	scope := date + "/" + s.cfg.Region + "/s3/aws4_request"

	canonicalHeaders, signedHeaders := canonicalHeaders(req)
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		req.Header.Get("X-Amz-Date"),
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := awsSigningKey(s.cfg.SecretKey, date, s.cfg.Region, "s3")
	signature := hmacHex(signingKey, []byte(stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKey,
		scope,
		signedHeaders,
		signature,
	))
}

func canonicalHeaders(req *http.Request) (string, string) {
	headers := map[string]string{
		"host":                 req.URL.Host,
		"x-amz-content-sha256": req.Header.Get("X-Amz-Content-Sha256"),
		"x-amz-date":           req.Header.Get("X-Amz-Date"),
	}
	if contentType := req.Header.Get("Content-Type"); contentType != "" {
		headers["content-type"] = contentType
	}

	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	var canonical strings.Builder
	for _, name := range names {
		canonical.WriteString(name)
		canonical.WriteByte(':')
		canonical.WriteString(strings.Join(strings.Fields(headers[name]), " "))
		canonical.WriteByte('\n')
	}
	return canonical.String(), strings.Join(names, ";")
}

func awsSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, value []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(value)
	return mac.Sum(nil)
}

func hmacHex(key, value []byte) string {
	return hex.EncodeToString(hmacSHA256(key, value))
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (s *S3Storage) responseError(resp *http.Response, action string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("%s failed: %s", action, message)
}
