package observe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	dlog "github.com/shemic/dever/log"
	"github.com/shemic/dever/util"
)

const (
	defaultHTTPJSONTimeout = 3 * time.Second
	defaultHTTPJSONBuffer  = 512
)

type httpJSONProvider struct {
	client   *http.Client
	endpoint string
	headers  map[string]string
	queue    chan Snapshot
	dropped  atomic.Uint64
	closed   atomic.Bool
	stop     chan struct{}
	done     chan struct{}
}

type httpJSONPayload struct {
	Kind         Kind           `json:"kind"`
	Service      string         `json:"service"`
	Name         string         `json:"name"`
	TraceID      string         `json:"traceId"`
	SpanID       string         `json:"spanId"`
	ParentSpanID string         `json:"parentSpanId,omitempty"`
	StartedAt    time.Time      `json:"startedAt"`
	EndedAt      time.Time      `json:"endedAt"`
	DurationMS   int64          `json:"durationMs"`
	Error        string         `json:"error,omitempty"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}

func init() {
	Register("http", newHTTPJSONProvider)
	Register("httpjson", newHTTPJSONProvider)
	Register("webhook", newHTTPJSONProvider)
}

func newHTTPJSONProvider(cfg Config) (Provider, error) {
	endpoint := firstObserveOptionString(cfg.Options, "endpoint", "url")
	if endpoint == "" {
		return nil, fmt.Errorf("http observe provider 缺少 endpoint/url 配置")
	}

	bufferSize := observeOptionInt(cfg.Options, defaultHTTPJSONBuffer, "buffer", "queue", "queueSize")
	if bufferSize <= 0 {
		bufferSize = defaultHTTPJSONBuffer
	}

	timeout := observeOptionDuration(cfg.Options, defaultHTTPJSONTimeout, "timeout")
	if timeout <= 0 {
		timeout = defaultHTTPJSONTimeout
	}

	provider := &httpJSONProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		endpoint: endpoint,
		headers:  observeOptionHeaders(cfg.Options),
		queue:    make(chan Snapshot, bufferSize),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go provider.loop()
	return provider, nil
}

func (p *httpJSONProvider) OnStart(context.Context, Snapshot) {}

func (p *httpJSONProvider) OnFinish(_ context.Context, snapshot Snapshot) {
	if p.closed.Load() {
		return
	}
	snapshot = cloneSnapshot(snapshot)
	endpoint := p.endpoint

	select {
	case <-p.stop:
		return
	default:
	}

	select {
	case p.queue <- snapshot:
		return
	case <-p.stop:
		return
	default:
	}

	dropped := p.dropped.Add(1)
	if dropped == 1 || dropped%100 == 0 {
		dlog.ErrorFields("observe_http_provider", "observe queue dropped", dlog.Fields{
			"provider": "http",
			"dropped":  dropped,
			"reason":   "queue_full",
			"endpoint": endpoint,
		})
	}
}

func (p *httpJSONProvider) loop() {
	defer close(p.done)
	for {
		select {
		case snapshot := <-p.queue:
			p.postAndLog(snapshot)
		case <-p.stop:
			p.drain()
			return
		}
	}
}

func (p *httpJSONProvider) Shutdown(ctx context.Context) error {
	ctx = normalizeContext(ctx)

	if p.closed.CompareAndSwap(false, true) {
		close(p.stop)
	}
	done := p.done

	select {
	case <-done:
		p.client.CloseIdleConnections()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *httpJSONProvider) drain() {
	for {
		select {
		case snapshot := <-p.queue:
			p.postAndLog(snapshot)
		default:
			return
		}
	}
}

func (p *httpJSONProvider) postAndLog(snapshot Snapshot) {
	if err := p.post(snapshot); err != nil {
		dlog.ErrorFields("observe_http_provider", "observe push failed", dlog.Fields{
			"provider": "http",
			"endpoint": p.endpoint,
			"error":    dlog.ErrorValue(err),
		})
	}
}

func (p *httpJSONProvider) post(snapshot Snapshot) error {
	body, err := json.Marshal(buildHTTPJSONPayload(snapshot))
	if err != nil {
		return fmt.Errorf("编码观测数据失败: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建观测请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range p.headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送观测请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("观测服务返回异常状态: %s", resp.Status)
	}
	return nil
}

func buildHTTPJSONPayload(snapshot Snapshot) httpJSONPayload {
	payload := httpJSONPayload{
		Kind:         snapshot.Kind,
		Service:      snapshot.Service,
		Name:         snapshot.Name,
		TraceID:      snapshot.TraceID,
		SpanID:       snapshot.SpanID,
		ParentSpanID: snapshot.ParentSpanID,
		StartedAt:    snapshot.StartedAt,
		EndedAt:      snapshot.EndedAt,
		DurationMS:   snapshot.Duration.Milliseconds(),
		Attributes:   snapshot.Attributes,
	}
	if snapshot.Error != nil {
		payload.Error = snapshot.Error.Error()
	}
	return payload
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	cloned.Attributes = util.CloneMap(snapshot.Attributes)
	return cloned
}

func firstObserveOptionString(options map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := util.ToStringTrimmed(options[key]); value != "" {
			return value
		}
	}
	return ""
}

func observeOptionInt(options map[string]any, fallback int, keys ...string) int {
	for _, key := range keys {
		if value, ok := options[key]; ok {
			return util.ToIntDefault(value, fallback)
		}
	}
	return fallback
}

func observeOptionDuration(options map[string]any, fallback time.Duration, keys ...string) time.Duration {
	for _, key := range keys {
		raw, ok := options[key]
		if !ok || raw == nil {
			continue
		}
		text := util.ToStringTrimmed(raw)
		if text != "" {
			if duration, err := time.ParseDuration(text); err == nil {
				return duration
			}
		}
		if nanos, ok := util.ParseInt64(raw); ok && nanos > 0 {
			return time.Duration(nanos)
		}
	}
	return fallback
}

func observeOptionHeaders(options map[string]any) map[string]string {
	raw, ok := options["headers"]
	if !ok || raw == nil {
		return nil
	}

	switch current := raw.(type) {
	case map[string]string:
		if len(current) == 0 {
			return nil
		}
		headers := make(map[string]string, len(current))
		for key, value := range current {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				continue
			}
			headers[key] = value
		}
		if len(headers) == 0 {
			return nil
		}
		return headers
	case map[string]any:
		headers := make(map[string]string, len(current))
		for key, value := range current {
			headerKey := strings.TrimSpace(key)
			headerValue := util.ToStringTrimmed(value)
			if headerKey == "" || headerValue == "" {
				continue
			}
			headers[headerKey] = headerValue
		}
		if len(headers) == 0 {
			return nil
		}
		return headers
	default:
		return nil
	}
}
