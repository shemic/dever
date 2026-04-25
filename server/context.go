package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/shemic/dever/util"
)

var errNoJSONMethod = errors.New("context: JSON method not found")

// JSONAdapter 用于扩展不同 HTTP 框架的 JSON 输出能力。
type JSONAdapter func(raw any, status int, data any) (handled bool, err error)

var (
	jsonAdapters      []JSONAdapter
	jsonAdapterMux    sync.Mutex
	jsonAdapterStored atomic.Value // stores []JSONAdapter

	validatorCache util.ConcurrentMap[string, validatorCacheEntry]

	// payloadPool reduces GC pressure by reusing payload maps
	payloadPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 4)
		},
	}

	// methodCache caches reflected methods to avoid repeated reflection lookups
	methodCache util.ConcurrentMap[reflect.Type, *cachedMethods]
)

// cachedMethods holds commonly used methods for a type
type cachedMethods struct {
	query     *reflect.Method
	params    *reflect.Method
	formValue *reflect.Method
	status    *reflect.Method
	json      *reflect.Method
}

// RegisterJSONAdapter 注册自定义 JSON 输出的扩展逻辑，例如 Fiber、Gin 等。
func RegisterJSONAdapter(fn JSONAdapter) {
	if fn == nil {
		return
	}
	jsonAdapterMux.Lock()
	defer jsonAdapterMux.Unlock()
	next := make([]JSONAdapter, 0, len(jsonAdapters)+1)
	next = append(next, jsonAdapters...)
	next = append(next, fn)
	jsonAdapters = next
	jsonAdapterStored.Store(next)
}

func callJSONAdapters(raw any, status int, data any) (bool, error) {
	loaded := jsonAdapterStored.Load()
	if loaded == nil {
		return false, nil
	}
	for _, adapter := range loaded.([]JSONAdapter) {
		if handled, err := adapter(raw, status, data); handled {
			return true, err
		} else if err != nil {
			return true, err
		}
	}
	return false, nil
}

// Context 封装底层框架的上下文（例如 *fiber.Ctx），对外暴露统一接口。
type Context struct {
	Raw          any
	jsonPayload  map[string]any
	jsonOnce     sync.Once
	baseCtx      context.Context
	baseCtxReady bool
}

// Context 返回底层请求关联的 context.Context。
func (c *Context) Context() context.Context {
	if c == nil {
		return context.Background()
	}
	if c.baseCtxReady {
		if c.baseCtx != nil {
			return c.baseCtx
		}
		return context.Background()
	}

	var resolved context.Context
	if raw := c.Raw; raw != nil {
		if v, ok := raw.(interface{ UserContext() context.Context }); ok {
			if ctx := v.UserContext(); ctx != nil {
				resolved = ctx
				goto done
			}
		}
		if v, ok := raw.(interface{ Context() context.Context }); ok {
			if ctx := v.Context(); ctx != nil {
				resolved = ctx
				goto done
			}
		}
		if v, ok := raw.(interface{ Request() *http.Request }); ok {
			if req := v.Request(); req != nil {
				if ctx := req.Context(); ctx != nil {
					resolved = ctx
					goto done
				}
			}
		}
	}

done:
	c.baseCtx = resolved
	c.baseCtxReady = true
	if resolved != nil {
		return resolved
	}
	return context.Background()
}

// AttachContext 将新的 context 绑定回底层 HTTP 框架上下文。
func AttachContext(raw any, ctx context.Context) {
	if raw == nil || ctx == nil {
		return
	}
	if v, ok := raw.(interface{ SetUserContext(context.Context) }); ok {
		v.SetUserContext(ctx)
		return
	}
	if v, ok := raw.(interface{ SetContext(context.Context) }); ok {
		v.SetContext(ctx)
		return
	}
	if v, ok := raw.(interface{ SetRequest(*http.Request) }); ok {
		if req, ok := raw.(interface{ Request() *http.Request }); ok && req.Request() != nil {
			v.SetRequest(req.Request().WithContext(ctx))
		}
	}
}

// SetContext 将新的 context 绑定到统一 Context，并同步回底层 HTTP 框架。
func (c *Context) SetContext(ctx context.Context) {
	if c == nil || ctx == nil {
		return
	}
	c.baseCtx = ctx
	c.baseCtxReady = true
	AttachContext(c.Raw, ctx)
}

// Abort 表示请求已在处理中断，通常由 Context 自动输出响应后触发。
type Abort struct {
	Err error
}

// BindJSON 将请求体绑定到目标结构体。
func (c *Context) BindJSON(v any) error {
	if c == nil || c.Raw == nil {
		return errors.New("BindJSON: nil context")
	}
	switch ctx := c.Raw.(type) {
	case interface{ BodyParser(any) error }:
		return ctx.BodyParser(v)
	case interface{ Body() []byte }:
		return json.Unmarshal(ctx.Body(), v)
	default:
		return errors.New("BindJSON: not supported framework")
	}
}

// JSON 输出 200 状态码的 JSON 结果。
func (c *Context) JSON(data any) error {
	return c.JSONWithStatus(http.StatusOK, data)
}

func (c *Context) writeJSONPayload(status int, payload map[string]any) error {
	if c.Raw == nil {
		return errors.New("JSON: nil context")
	}

	if handled, err := callJSONAdapters(c.Raw, status, payload); handled {
		return err
	}
	setStatusIfPossible(c.Raw, status)
	switch ctx := c.Raw.(type) {
	case interface{ JSON(any) error }:
		return ctx.JSON(payload)
	case interface{ JSON(int, any) error }:
		return ctx.JSON(status, payload)
	}
	if err := callJSONReflect(c.Raw, status, payload); err != nil {
		if errors.Is(err, errNoJSONMethod) {
			return errors.New("JSON: not supported framework")
		}
		return err
	}
	return nil
}

// JSONPayload 输出已归一化的 payload，避免重复包装。
func (c *Context) JSONPayload(status int, payload map[string]any) error {
	return c.writeJSONPayload(status, payload)
}

// JSONWithStatus 输出指定状态码的 JSON 结果。
func (c *Context) JSONWithStatus(status int, data any) error {
	payload := normalizePayload(status, data)
	defer releasePayload(payload)
	return c.writeJSONPayload(status, payload)
}

// Error 将错误信息以 JSON 输出，默认状态码 400，可自定义。
func (c *Context) Error(err any, code ...int) error {
	status := http.StatusBadRequest
	if len(code) > 0 {
		status = code[0]
	}

	var e error
	switch v := err.(type) {
	case nil:
		e = errors.New(http.StatusText(status))
	case error:
		e = v
	case string:
		e = errors.New(v)
	default:
		e = fmt.Errorf("%v", v)
	}

	payload := normalizeErrorPayload(status, e)
	defer releasePayload(payload)
	if jErr := c.writeJSONPayload(status, payload); jErr != nil {
		return jErr
	}
	return nil
}

func normalizePayload(status int, data any) map[string]any {
	payloadStatus, message := resolvePayloadStatus(status)
	payload := acquirePayload()
	payload["code"] = status
	payload["status"] = payloadStatus
	payload["msg"] = message
	payload["data"] = data
	return payload
}

func normalizeErrorPayload(status int, err error) map[string]any {
	payload := acquirePayload()
	payload["code"] = status
	payload["status"] = 2
	payload["data"] = nil
	if err != nil && err.Error() != "" {
		payload["msg"] = err.Error()
	} else if txt := http.StatusText(status); txt != "" {
		payload["msg"] = txt
	} else {
		payload["msg"] = "error"
	}
	return payload
}

func acquirePayload() map[string]any {
	payload := payloadPool.Get().(map[string]any)
	for k := range payload {
		delete(payload, k)
	}
	return payload
}

func releasePayload(payload map[string]any) {
	if payload == nil {
		return
	}
	for k := range payload {
		delete(payload, k)
	}
	payloadPool.Put(payload)
}

func resolvePayloadStatus(status int) (int, string) {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return 1, "success"
	}
	if txt := http.StatusText(status); txt != "" {
		return 2, txt
	}
	return 2, "error"
}

// Input 统一读取请求参数，按路径参数→查询参数→表单参数的顺序查找。
// args 可选，依次为：验证正则表达式、参数描述、默认值。
func (c *Context) Input(key string, args ...string) string {
	if c == nil {
		panic("Input: nil context")
	}

	var (
		rule       string
		desc       string
		defaultVal string
	)
	if len(args) > 0 {
		rule = strings.TrimSpace(args[0])
	}
	if len(args) > 1 {
		desc = strings.TrimSpace(args[1])
	}
	if len(args) > 2 {
		defaultVal = args[2]
	}
	if desc == "" {
		desc = key
	}

	var value string
	if v, ok := lookupParamsString(c.Raw, key); ok && v != "" {
		value = v
	}
	if value == "" {
		if v, ok := lookupQueryString(c.Raw, key); ok && v != "" {
			value = v
		}
	}
	if value == "" {
		if v, ok := lookupFormValueString(c.Raw, key); ok && v != "" {
			value = v
		}
	}
	if value == "" {
		if payload := c.loadJSONPayload(); payload != nil {
			if v, ok := payload[key]; ok {
				switch vv := v.(type) {
				case string:
					value = strings.TrimSpace(vv)
				case bool:
					value = strconv.FormatBool(vv)
				case json.Number:
					value = vv.String()
				case float64:
					value = strconv.FormatFloat(vv, 'f', -1, 64)
				case float32:
					value = strconv.FormatFloat(float64(vv), 'f', -1, 32)
				case int:
					value = strconv.Itoa(vv)
				case int8:
					value = strconv.FormatInt(int64(vv), 10)
				case int16:
					value = strconv.FormatInt(int64(vv), 10)
				case int32:
					value = strconv.FormatInt(int64(vv), 10)
				case int64:
					value = strconv.FormatInt(vv, 10)
				case uint:
					value = strconv.FormatUint(uint64(vv), 10)
				case uint8:
					value = strconv.FormatUint(uint64(vv), 10)
				case uint16:
					value = strconv.FormatUint(uint64(vv), 10)
				case uint32:
					value = strconv.FormatUint(uint64(vv), 10)
				case uint64:
					value = strconv.FormatUint(vv, 10)
				default:
					if b, err := json.Marshal(v); err == nil {
						value = strings.TrimSpace(string(b))
					} else {
						value = util.ToStringTrimmed(v)
					}
				}
			}
		}
	}
	if value == "" {
		value = defaultVal
	}
	value = strings.TrimSpace(value)

	// Validation logic - rule already trimmed above
	if rule != "" {
		lowerRule := strings.ToLower(rule)
		if lowerRule == "required" || lowerRule == "is_required" {
			if value == "" {
				c.abort(fmt.Errorf("%s 不能为空", desc))
			}
		} else if value != "" {
			re, err := compileValidator(rule)
			if err != nil {
				c.abort(fmt.Errorf("%s 校验规则错误: %v", desc, err))
			} else if !re.MatchString(value) {
				c.abort(fmt.Errorf("%s 格式不正确", desc))
			}
		}
	}

	return value
}

func (c *Context) loadJSONPayload() map[string]any {
	if c == nil {
		return nil
	}
	c.jsonOnce.Do(func() {
		if c.Raw == nil {
			return
		}
		if v, ok := c.Raw.(interface{ Body() []byte }); ok {
			body := bytes.TrimSpace(v.Body())
			if len(body) == 0 || body[0] != '{' {
				return
			}
			payload := make(map[string]any)
			if err := json.Unmarshal(body, &payload); err != nil {
				return
			}
			if len(payload) > 0 {
				c.jsonPayload = payload
			}
			return
		}
		payload := make(map[string]any)
		if err := c.BindJSON(&payload); err != nil {
			return
		}
		if len(payload) > 0 {
			c.jsonPayload = payload
		}
	})
	return c.jsonPayload
}

// Query 返回查询字符串中的参数值，不存在时返回默认值或空字符串。
func (c *Context) Query(key string, defaultValue ...string) string {
	if c == nil || c.Raw == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}

	val, ok := lookupQueryString(c.Raw, key, defaultValue...)
	if ok && val != "" {
		return val
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

// Method 返回请求的 HTTP 方法。
func (c *Context) Method() string {
	if c == nil || c.Raw == nil {
		return ""
	}
	if m, ok := c.Raw.(interface{ Method() string }); ok {
		return m.Method()
	}
	if m, ok := c.Raw.(interface{ Method(...string) string }); ok {
		return m.Method()
	}
	if v, ok := c.Raw.(interface{ Request() *http.Request }); ok {
		if req := v.Request(); req != nil {
			return req.Method
		}
	}
	return ""
}

// Path 返回请求路径。
func (c *Context) Path() string {
	if c == nil || c.Raw == nil {
		return ""
	}
	if r, ok := c.Raw.(interface{ Path() string }); ok {
		return r.Path()
	}
	if a, ok := c.Raw.(interface{ OriginalURL() string }); ok {
		return a.OriginalURL()
	}
	return ""
}

// Header 返回请求头值，不存在时返回默认值或空字符串。
func (c *Context) Header(key string, defaultValue ...string) string {
	if c == nil || c.Raw == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}

	if getter, ok := c.Raw.(interface {
		Get(string, ...string) string
	}); ok {
		value := getter.Get(key)
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	if v, ok := c.Raw.(interface{ Request() *http.Request }); ok {
		if req := v.Request(); req != nil {
			if value := req.Header.Get(key); strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func getCachedMethods(t reflect.Type) *cachedMethods {
	if cached, ok := methodCache.Load(t); ok {
		return cached
	}

	cm := &cachedMethods{}
	if m, ok := t.MethodByName("Query"); ok {
		cm.query = &m
	}
	if m, ok := t.MethodByName("Params"); ok {
		cm.params = &m
	}
	if m, ok := t.MethodByName("FormValue"); ok {
		cm.formValue = &m
	}
	if m, ok := t.MethodByName("Status"); ok {
		cm.status = &m
	}
	if m, ok := t.MethodByName("JSON"); ok {
		cm.json = &m
	}
	methodCache.Store(t, cm)
	return cm
}

func cachedMethodValue(raw any, name string) (reflect.Value, bool) {
	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return reflect.Value{}, false
	}

	cached := getCachedMethods(rv.Type())
	var method *reflect.Method
	switch name {
	case "Query":
		method = cached.query
	case "Params":
		method = cached.params
	case "FormValue":
		method = cached.formValue
	case "Status":
		method = cached.status
	case "JSON":
		method = cached.json
	}
	if method == nil {
		return reflect.Value{}, false
	}
	return rv.Method(method.Index), true
}

func lookupParamsString(raw any, key string, defaultValue ...string) (string, bool) {
	if raw == nil {
		return "", false
	}

	if ctx, ok := raw.(interface {
		Params(string, ...string) string
	}); ok {
		return ctx.Params(key, defaultValue...), true
	}
	if ctx, ok := raw.(interface{ Params(string) string }); ok {
		return ctx.Params(key), true
	}
	return lookupCachedStringMethod(raw, "Params", key, defaultValue...)
}

func lookupQueryString(raw any, key string, defaultValue ...string) (string, bool) {
	if raw == nil {
		return "", false
	}

	if ctx, ok := raw.(interface {
		Query(string, ...string) string
	}); ok {
		return ctx.Query(key, defaultValue...), true
	}
	if ctx, ok := raw.(interface{ Query(string) string }); ok {
		return ctx.Query(key), true
	}
	return lookupCachedStringMethod(raw, "Query", key, defaultValue...)
}

func lookupFormValueString(raw any, key string) (string, bool) {
	if raw == nil {
		return "", false
	}

	if ctx, ok := raw.(interface{ FormValue(string) string }); ok {
		return ctx.FormValue(key), true
	}
	return lookupCachedStringMethod(raw, "FormValue", key)
}

func lookupCachedStringMethod(raw any, methodName, key string, defaultValue ...string) (string, bool) {
	method, ok := cachedMethodValue(raw, methodName)
	if !ok {
		return "", false
	}

	val := callStringMethod(method, key, defaultValue...)
	return val, true
}

func setStatusIfPossible(raw any, status int) {
	switch ctx := raw.(type) {
	case interface{ Status(int) any }:
		ctx.Status(status)
		return
	case interface{ Status(int) }:
		ctx.Status(status)
		return
	}

	method, ok := cachedMethodValue(raw, "Status")
	if !ok {
		return
	}

	if method.Type().NumIn() != 1 || method.Type().In(0).Kind() != reflect.Int {
		return
	}
	method.Call([]reflect.Value{reflect.ValueOf(status)})
}

func callJSONReflect(raw any, status int, data any) (err error) {
	defer func() {
		if recover() != nil {
			err = errNoJSONMethod
		}
	}()

	method, ok := cachedMethodValue(raw, "JSON")
	if !ok {
		return errNoJSONMethod
	}

	switch method.Type().NumIn() {
	case 1:
		arg, ok := reflectArg(method.Type().In(0), data)
		if !ok {
			return errNoJSONMethod
		}
		results := method.Call([]reflect.Value{arg})
		return extractError(results)
	case 2:
		if method.Type().In(0).Kind() != reflect.Int {
			return errNoJSONMethod
		}
		arg, ok := reflectArg(method.Type().In(1), data)
		if !ok {
			return errNoJSONMethod
		}
		results := method.Call([]reflect.Value{
			reflect.ValueOf(status),
			arg,
		})
		return extractError(results)
	default:
		return errNoJSONMethod
	}
}

func reflectArg(target reflect.Type, value any) (reflect.Value, bool) {
	if target == nil {
		return reflect.Value{}, false
	}
	if value == nil {
		switch target.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			return reflect.Zero(target), true
		default:
			return reflect.Value{}, false
		}
	}
	current := reflect.ValueOf(value)
	if current.Type().AssignableTo(target) {
		return current, true
	}
	if current.Type().ConvertibleTo(target) {
		return current.Convert(target), true
	}
	return reflect.Value{}, false
}

func extractError(results []reflect.Value) error {
	if len(results) == 0 {
		return nil
	}
	if errVal, ok := results[0].Interface().(error); ok {
		return errVal
	}
	return nil
}

func callStringMethod(method reflect.Value, key string, defaultValue ...string) (result string) {
	fallback := ""
	if len(defaultValue) > 0 {
		fallback = defaultValue[0]
	}
	defer func() {
		if recover() != nil {
			result = fallback
		}
	}()

	methodType := method.Type()
	if methodType.NumIn() == 0 || methodType.In(0).Kind() != reflect.String {
		return fallback
	}
	args := []reflect.Value{reflect.ValueOf(key)}

	if methodType.IsVariadic() {
		if len(defaultValue) > 0 && methodType.NumIn() > 1 && methodType.In(methodType.NumIn()-1).Elem().Kind() == reflect.String {
			args = append(args, reflect.ValueOf(defaultValue[0]))
		}
	} else {
		switch methodType.NumIn() {
		case 1:
			// only key argument
		case 2:
			if len(defaultValue) > 0 && methodType.In(1).Kind() == reflect.String {
				args = append(args, reflect.ValueOf(defaultValue[0]))
			} else {
				args = append(args, reflect.Zero(methodType.In(1)))
			}
		default:
			return fallback
		}
	}

	results := method.Call(args)
	if len(results) == 0 {
		return fallback
	}
	if str, ok := results[0].Interface().(string); ok {
		if str == "" {
			return fallback
		}
		return str
	}
	return fallback
}

type validatorCacheEntry struct {
	re  *regexp.Regexp
	err error
}

func compileValidator(rule string) (*regexp.Regexp, error) {
	trimmed := strings.TrimSpace(rule)
	var pattern string
	var cacheKey string

	switch strings.ToLower(trimmed) {
	case "is_string":
		cacheKey = "is_string"
		pattern = `^[\p{L}\p{N}_\-\s]+$`
	case "is_number":
		cacheKey = "is_number"
		pattern = `^-?\d+(?:\.\d+)?$`
	default:
		cacheKey = trimmed
		pattern = trimmed
	}

	if cached, ok := validatorCache.Load(cacheKey); ok {
		return cached.re, cached.err
	}

	re, err := regexp.Compile(pattern)
	validatorCache.Store(cacheKey, validatorCacheEntry{re: re, err: err})
	return re, err
}

func (c *Context) abort(err error) {
	if err == nil {
		return
	}
	_ = c.Error(err)
	panic(Abort{Err: err})
}
