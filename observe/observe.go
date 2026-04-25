package observe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Kind string

const (
	KindRequest Kind = "request"
	KindDB      Kind = "db"
)

type Config struct {
	Enabled     bool
	Provider    string
	Service     string
	SlowRequest time.Duration
	SlowSQL     time.Duration
	Options     map[string]any
}

type Snapshot struct {
	Kind         Kind
	Service      string
	Name         string
	TraceID      string
	SpanID       string
	ParentSpanID string
	StartedAt    time.Time
	EndedAt      time.Time
	Duration     time.Duration
	Error        error
	Attributes   map[string]any
}

type Provider interface {
	OnStart(ctx context.Context, snapshot Snapshot)
	OnFinish(ctx context.Context, snapshot Snapshot)
}

type lifecycleProvider interface {
	Shutdown(ctx context.Context) error
}

type ProviderFactory func(Config) (Provider, error)

type Span interface {
	SetAttribute(key string, value any)
	RecordError(err error)
	End()
}

type contextMeta struct {
	traceID     string
	spanID      string
	currentSpan *runtimeSpan
}

type runtimeConfig struct {
	enabled  bool
	service  string
	provider []Provider
}

type runtimeSpan struct {
	once     sync.Once
	cfg      runtimeConfig
	ctx      context.Context
	snapshot Snapshot
}

type spanContextKey struct{}

type registryState struct {
	mu        sync.RWMutex
	factories map[string]ProviderFactory
}

var (
	registry = registryState{
		factories: map[string]ProviderFactory{},
	}
	currentConfig atomic.Value // runtimeConfig
	fallbackID    atomic.Uint64
)

func init() {
	currentConfig.Store(runtimeConfig{})
}

func Register(name string, factory ProviderFactory) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || factory == nil {
		return
	}
	registry.mu.Lock()
	registry.factories[name] = factory
	registry.mu.Unlock()
}

func Configure(cfg Config) error {
	previous := current()
	normalized := runtimeConfig{
		enabled: cfg.Enabled,
		service: strings.TrimSpace(cfg.Service),
	}
	if normalized.service == "" {
		normalized.service = "Dever"
	}
	if !cfg.Enabled {
		currentConfig.Store(normalized)
		return shutdownProviders(context.Background(), previous.provider)
	}

	providers := []Provider{newBuiltinProvider(cfg)}
	providerName := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if providerName != "" && providerName != "builtin" && providerName != "self" {
		factory, ok := loadFactory(providerName)
		if !ok {
			return fmt.Errorf("observe provider 未注册: %s", providerName)
		}
		provider, err := factory(cfg)
		if err != nil {
			return fmt.Errorf("observe provider 初始化失败: %w", err)
		}
		if provider != nil {
			providers = append(providers, provider)
		}
	}

	normalized.provider = providers
	currentConfig.Store(normalized)
	return shutdownProviders(context.Background(), previous.provider)
}

func Start(ctx context.Context, kind Kind, name string, attrs map[string]any) (context.Context, Span) {
	cfg := current()
	if !cfg.enabled {
		if ctx == nil {
			ctx = context.Background()
		}
		return ctx, noopSpan{}
	}

	ctx = normalizeContext(ctx)
	parent := contextMetaFrom(ctx)
	spanID := newSpanID()
	traceID := parent.traceID
	parentSpanID := parent.spanID
	if traceID == "" {
		traceID = newTraceID()
	}

	snapshot := Snapshot{
		Kind:         kind,
		Service:      cfg.service,
		Name:         strings.TrimSpace(name),
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		StartedAt:    time.Now(),
		Attributes:   cloneAttributes(attrs),
	}

	span := &runtimeSpan{
		cfg:      cfg,
		snapshot: snapshot,
	}
	ctx = context.WithValue(ctx, spanContextKey{}, contextMeta{
		traceID:     traceID,
		spanID:      spanID,
		currentSpan: span,
	})
	span.ctx = ctx

	for _, provider := range cfg.provider {
		provider.OnStart(ctx, snapshot)
	}
	return ctx, span
}

func SetAttribute(ctx context.Context, key string, value any) {
	if span := currentSpan(ctx); span != nil {
		span.SetAttribute(key, value)
	}
}

func RecordError(ctx context.Context, err error) {
	if span := currentSpan(ctx); span != nil {
		span.RecordError(err)
	}
}

func TraceID(ctx context.Context) string {
	return contextMetaFrom(ctx).traceID
}

func SpanID(ctx context.Context) string {
	return contextMetaFrom(ctx).spanID
}

func Enabled() bool {
	return current().enabled
}

func Shutdown(ctx context.Context) error {
	return shutdownProviders(ctx, current().provider)
}

func (s *runtimeSpan) SetAttribute(key string, value any) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if s.snapshot.Attributes == nil {
		s.snapshot.Attributes = map[string]any{}
	}
	s.snapshot.Attributes[key] = value
}

func (s *runtimeSpan) RecordError(err error) {
	if err == nil {
		return
	}
	s.snapshot.Error = err
}

func (s *runtimeSpan) End() {
	s.once.Do(func() {
		s.snapshot.EndedAt = time.Now()
		s.snapshot.Duration = s.snapshot.EndedAt.Sub(s.snapshot.StartedAt)
		snapshot := s.snapshot

		for _, provider := range s.cfg.provider {
			provider.OnFinish(s.ctx, snapshot)
		}
	})
}

func loadFactory(name string) (ProviderFactory, bool) {
	registry.mu.RLock()
	factory, ok := registry.factories[name]
	registry.mu.RUnlock()
	return factory, ok
}

func current() runtimeConfig {
	if loaded := currentConfig.Load(); loaded != nil {
		if cfg, ok := loaded.(runtimeConfig); ok {
			return cfg
		}
	}
	return runtimeConfig{}
}

func shutdownProviders(ctx context.Context, providers []Provider) error {
	if len(providers) == 0 {
		return nil
	}
	ctx = normalizeContext(ctx)
	var errs []error
	for _, provider := range providers {
		lifecycle, ok := provider.(lifecycleProvider)
		if !ok || lifecycle == nil {
			continue
		}
		if err := lifecycle.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func contextMetaFrom(ctx context.Context) contextMeta {
	if ctx == nil {
		return contextMeta{}
	}
	if meta, ok := ctx.Value(spanContextKey{}).(contextMeta); ok {
		return meta
	}
	return contextMeta{}
}

func currentSpan(ctx context.Context) *runtimeSpan {
	return contextMetaFrom(ctx).currentSpan
}

func cloneAttributes(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(attrs))
	for key, value := range attrs {
		cloned[key] = value
	}
	return cloned
}

func newTraceID() string {
	return randomHexID(16)
}

func newSpanID() string {
	return randomHexID(8)
}

func randomHexID(byteLen int) string {
	if byteLen <= 0 {
		return ""
	}

	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	counter := fallbackID.Add(1)
	raw := fmt.Sprintf("%016x%016x", time.Now().UnixNano(), counter)
	targetLen := byteLen * 2
	if len(raw) >= targetLen {
		return raw[:targetLen]
	}
	return strings.Repeat("0", targetLen-len(raw)) + raw
}

type noopSpan struct{}

func (noopSpan) SetAttribute(string, any) {}
func (noopSpan) RecordError(error)        {}
func (noopSpan) End()                     {}
