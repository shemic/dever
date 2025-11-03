package lock

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	mu          sync.RWMutex
	redisClient *redis.Client
	keyPrefix   string
	enabled     bool
	configSet   bool
	cachedConfig Config

	errFloorViolation = "ERR_FLOOR"
	errCeilViolation  = "ERR_CEIL"
	errBadValue       = "ERR_BADVALUE"
)

var adjustScript = redis.NewScript(`
local key = KEYS[1]
local delta = tonumber(ARGV[1])
local floor = tonumber(ARGV[2])
local hasCeil = tonumber(ARGV[3])
local ceil = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local current = redis.call('GET', key)
if not current then
	current = 0
else
	current = tonumber(current)
	if not current then
		return "` + errBadValue + `"
	end
end

local next = current + delta
if next < floor then
	return "` + errFloorViolation + `"
end
if hasCeil == 1 and next > ceil then
	return "` + errCeilViolation + `"
end

local value = redis.call('INCRBY', key, delta)
if ttl > 0 then
	redis.call('PEXPIRE', key, ttl)
elseif ttl == -1 then
	redis.call('PERSIST', key)
end
return value
`)

// Config 描述 Redis 客户端初始化参数。
type Config struct {
	Enable       bool
	Addr         string
	Username     string
	Password     string
	DB           int
	Prefix       string
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingTimeout  time.Duration
	UseTLS       bool
}

// ErrDisabled 表示 Redis 客户端未初始化。
var ErrDisabled = errors.New("lock: redis client disabled")

// ErrInsufficient 表示当前值不足以完成扣减。
var ErrInsufficient = errors.New("lock: insufficient quota")

// ErrOverflow 表示递增后超过配置的上限。
var ErrOverflow = errors.New("lock: exceed ceiling")

// ErrNonNumeric 表示 Redis 中存储的值无法解析为整数。
var ErrNonNumeric = errors.New("lock: value is not numeric")

type operationOptions struct {
	floor    int64
	ceil     *int64
	ttl      time.Duration
	ttlSet   bool
}

// Option 定义原子操作的可选参数。
type Option func(*operationOptions)

// WithFloor 设置允许的最小值，默认 Dec 操作为 0，Inc 为 math.MinInt64。
func WithFloor(min int64) Option {
	return func(o *operationOptions) {
		o.floor = min
	}
}

// WithCeiling 设置操作后的最大值，仅对 Inc 生效。
func WithCeiling(max int64) Option {
	return func(o *operationOptions) {
		o.ceil = &max
	}
}

// WithTTL 设置结果的过期时间；传入 0 或负数表示移除 TTL。
func WithTTL(ttl time.Duration) Option {
	return func(o *operationOptions) {
		o.ttl = ttl
		o.ttlSet = true
	}
}

// Configure 仅记录配置，延迟到首次使用时再建立连接。
func Configure(conf Config) {
	mu.Lock()
	defer mu.Unlock()

	if redisClient != nil {
		_ = redisClient.Close()
		redisClient = nil
	}
	cachedConfig = conf
	configSet = true
	keyPrefix = strings.TrimSpace(conf.Prefix)
	enabled = false
}

// Init 使用配置初始化 Redis 客户端。
func Init(conf Config) error {
	Configure(conf)
	if !conf.Enable {
		return nil
	}
	_, err := getClient()
	return err
}

// Close 关闭 Redis 客户端。
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if redisClient == nil {
		enabled = false
		return nil
	}
	err := redisClient.Close()
	redisClient = nil
	enabled = false
	return err
}

// Enabled 返回 Redis 客户端是否已就绪。
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled && redisClient != nil
}

// Inc 执行原子递增，可选设置最大值。
func Inc(ctx context.Context, key string, delta int64, opts ...Option) (int64, error) {
	if delta < 0 {
		return 0, fmt.Errorf("lock: inc delta must be >= 0")
	}
	if delta == 0 {
		return Get(ctx, key)
	}
	options := operationOptions{
		floor: math.MinInt64,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return applyDelta(ctx, key, delta, options)
}

// Dec 扣减指定值，默认不允许小于 0。
func Dec(ctx context.Context, key string, delta int64, opts ...Option) (int64, error) {
	if delta <= 0 {
		return 0, fmt.Errorf("lock: dec delta must be > 0")
	}
	options := operationOptions{
		floor: 0,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return applyDelta(ctx, key, -delta, options)
}

// Get 读取当前值，若键不存在返回 0。
func Get(ctx context.Context, key string) (int64, error) {
	client, err := getClient()
	if err != nil {
		return 0, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rkey, err := buildKey(key)
	if err != nil {
		return 0, err
	}
	val, err := client.Get(ctx, rkey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	n, parseErr := parseInt(val)
	if parseErr != nil {
		return 0, parseErr
	}
	return n, nil
}

// Set 写入值，可选设置过期时间；默认保持原 TTL。
func Set(ctx context.Context, key string, value int64, ttl ...time.Duration) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rkey, err := buildKey(key)
	if err != nil {
		return err
	}
	expire := time.Duration(redis.KeepTTL)
	if len(ttl) > 0 {
		if ttl[0] > 0 {
			expire = ttl[0]
		} else {
			expire = 0
		}
	}
	return client.Set(ctx, rkey, value, expire).Err()
}

func applyDelta(ctx context.Context, key string, delta int64, options operationOptions) (int64, error) {
	client, err := getClient()
	if err != nil {
		return 0, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rkey, err := buildKey(key)
	if err != nil {
		return 0, err
	}
	hasCeil := 0
	ceilValue := int64(0)
	if options.ceil != nil {
		hasCeil = 1
		ceilValue = *options.ceil
	}

	ttlArg := int64(0)
	if options.ttlSet {
		if options.ttl <= 0 {
			ttlArg = -1
		} else {
			ttlArg = options.ttl.Milliseconds()
			if ttlArg <= 0 {
				ttlArg = 1
			}
		}
	}

	result, err := adjustScript.Run(ctx, client, []string{rkey}, delta, options.floor, hasCeil, ceilValue, ttlArg).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrInsufficient
		}
		return 0, err
	}
	switch v := result.(type) {
	case int64:
		return v, nil
	case string:
		switch v {
		case errFloorViolation:
			return 0, ErrInsufficient
		case errCeilViolation:
			return 0, ErrOverflow
		case errBadValue:
			return 0, ErrNonNumeric
		default:
			return 0, fmt.Errorf("lock: unexpected response %q", v)
		}
	default:
		return 0, fmt.Errorf("lock: unexpected result type %T", result)
	}
}

func createClient(conf Config) (*redis.Client, error) {
	if strings.TrimSpace(conf.Addr) == "" {
		return nil, fmt.Errorf("lock: redis addr required")
	}
	opts := &redis.Options{
		Addr:         strings.TrimSpace(conf.Addr),
		Username:     strings.TrimSpace(conf.Username),
		Password:     conf.Password,
		DB:           conf.DB,
		PoolSize:     conf.PoolSize,
		MinIdleConns: conf.MinIdleConns,
		MaxRetries:   conf.MaxRetries,
	}
	if conf.DialTimeout > 0 {
		opts.DialTimeout = conf.DialTimeout
	}
	if conf.ReadTimeout > 0 {
		opts.ReadTimeout = conf.ReadTimeout
	}
	if conf.WriteTimeout > 0 {
		opts.WriteTimeout = conf.WriteTimeout
	}
	if conf.UseTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)
	pingTimeout := conf.PingTimeout
	if pingTimeout <= 0 {
		pingTimeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("lock: redis ping failed: %w", err)
	}
	return client, nil
}

func getClient() (*redis.Client, error) {
	mu.RLock()
	client := redisClient
	cfgSet := configSet
	conf := cachedConfig
	mu.RUnlock()

	if client != nil {
		return client, nil
	}
	if !cfgSet || !conf.Enable {
		return nil, ErrDisabled
	}

	mu.Lock()
	defer mu.Unlock()
	if redisClient != nil {
		return redisClient, nil
	}
	if !configSet || !cachedConfig.Enable {
		return nil, ErrDisabled
	}
	newClient, err := createClient(cachedConfig)
	if err != nil {
		return nil, err
	}
	redisClient = newClient
	keyPrefix = strings.TrimSpace(cachedConfig.Prefix)
	enabled = true
	return redisClient, nil
}

func buildKey(key string) (string, error) {
	target := strings.TrimSpace(key)
	if target == "" {
		return "", fmt.Errorf("lock: key required")
	}
	if keyPrefix != "" {
		return keyPrefix + ":" + target, nil
	}
	return target, nil
}

func parseInt(val string) (int64, error) {
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, ErrNonNumeric
	}
	return n, nil
}
