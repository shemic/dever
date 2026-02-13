package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

var errNoJSONMethod = errors.New("context: JSON method not found")

// JSONAdapter 用于扩展不同 HTTP 框架的 JSON 输出能力。
type JSONAdapter func(raw any, status int, data any) (handled bool, err error)

var (
	jsonAdapters   []JSONAdapter
	jsonAdapterMux sync.RWMutex

	validatorCache sync.Map

	// payloadPool reduces GC pressure by reusing payload maps
	payloadPool = sync.Pool{
		New: func() any {
			return make(map[string]any, 4)
		},
	}

	// methodCache caches reflected methods to avoid repeated reflection lookups
	methodCache sync.Map // map[reflect.Type]cachedMethods
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
	jsonAdapterMux.Lock()
	defer jsonAdapterMux.Unlock()
	jsonAdapters = append(jsonAdapters, fn)
}

func callJSONAdapters(raw any, status int, data any) (bool, error) {
	jsonAdapterMux.RLock()
	defer jsonAdapterMux.RUnlock()
	for _, adapter := range jsonAdapters {
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
	Raw            any
	jsonPayload    map[string]any
	jsonPayloadErr error
	jsonOnce       sync.Once
}

// Context 返回底层请求关联的 context.Context。
func (c *Context) Context() context.Context {
	if c == nil {
		return context.Background()
	}
	if raw := c.Raw; raw != nil {
		if v, ok := raw.(interface{ UserContext() context.Context }); ok {
			if ctx := v.UserContext(); ctx != nil {
				return ctx
			}
		}
		if v, ok := raw.(interface{ Context() context.Context }); ok {
			if ctx := v.Context(); ctx != nil {
				return ctx
			}
		}
		if v, ok := raw.(interface{ Request() *http.Request }); ok {
			if req := v.Request(); req != nil {
				if ctx := req.Context(); ctx != nil {
					return ctx
				}
			}
		}
	}
	return context.Background()
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

// JSONPayload 输出已归一化的 payload，避免重复包装。
func (c *Context) JSONPayload(status int, payload map[string]any) error {
	if c.Raw == nil {
		return errors.New("JSON: nil context")
	}
	// Return payload to pool after JSON serialization completes
	defer func() {
		if payload != nil {
			payloadPool.Put(payload)
		}
	}()

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

// JSONWithStatus 输出指定状态码的 JSON 结果。
func (c *Context) JSONWithStatus(status int, data any) error {
	if c.Raw == nil {
		return errors.New("JSON: nil context")
	}

	payload := normalizePayload(status, data)

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
	if jErr := c.JSONPayload(status, payload); jErr != nil {
		return jErr
	}
	return nil
}

func normalizePayload(status int, data any) map[string]any {
	payloadStatus := 1
	message := "success"
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		payloadStatus = 2
		if txt := http.StatusText(status); txt != "" {
			message = txt
		} else {
			message = "error"
		}
	}
	payload := payloadPool.Get().(map[string]any)
	// Clear the map before reuse
	for k := range payload {
		delete(payload, k)
	}
	payload["code"] = status
	payload["status"] = payloadStatus
	payload["msg"] = message
	payload["data"] = data
	return payload
}

func normalizeErrorPayload(status int, err error) map[string]any {
	payload := payloadPool.Get().(map[string]any)
	// Clear the map before reuse
	for k := range payload {
		delete(payload, k)
	}
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
	if v, ok := lookupString(c.Raw, "Params", key); ok && v != "" {
		value = v
	}
	if value == "" {
		if v, ok := lookupString(c.Raw, "Query", key); ok && v != "" {
			value = v
		}
	}
	if value == "" {
		if v, ok := lookupString(c.Raw, "FormValue", key); ok && v != "" {
			value = v
		}
	}
	if value == "" {
		if payload := c.loadJSONPayload(); payload != nil {
			if v, ok := payload[key]; ok {
				switch vv := v.(type) {
				case string:
					value = strings.TrimSpace(vv)
				default:
					if b, err := json.Marshal(v); err == nil {
						value = strings.TrimSpace(string(b))
					} else {
						value = strings.TrimSpace(fmt.Sprint(v))
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
			// Only trim for empty check to avoid multiple TrimSpace calls
			if strings.TrimSpace(value) == "" {
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
		payload := make(map[string]any)
		if v, ok := c.Raw.(interface{ Body() []byte }); ok {
			if b := v.Body(); len(b) > 0 {
				if uErr := json.Unmarshal(b, &payload); uErr == nil {
					c.jsonPayload = payload
					return
				}
			}
		}
		if err := c.BindJSON(&payload); err != nil {
			c.jsonPayloadErr = err
			return
		}
		c.jsonPayload = payload
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

	val, ok := lookupString(c.Raw, "Query", key, defaultValue...)
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

// getCachedMethods retrieves or creates cached methods for a type
func getCachedMethods(t reflect.Type) *cachedMethods {
	if cached, ok := methodCache.Load(t); ok {
		return cached.(*cachedMethods)
	}

	cm := &cachedMethods{}

	// Cache Query method
	if m, ok := t.MethodByName("Query"); ok {
		cm.query = &m
	}

	// Cache Params method
	if m, ok := t.MethodByName("Params"); ok {
		cm.params = &m
	}

	// Cache FormValue method
	if m, ok := t.MethodByName("FormValue"); ok {
		cm.formValue = &m
	}

	// Cache Status method
	if m, ok := t.MethodByName("Status"); ok {
		cm.status = &m
	}

	// Cache JSON method
	if m, ok := t.MethodByName("JSON"); ok {
		cm.json = &m
	}

	methodCache.Store(t, cm)
	return cm
}

func lookupString(raw any, methodName, key string, defaultValue ...string) (string, bool) {
	if raw == nil {
		return "", false
	}

	switch methodName {
	case "Params":
		if ctx, ok := raw.(interface {
			Params(string, ...string) string
		}); ok {
			return ctx.Params(key, defaultValue...), true
		}
		if ctx, ok := raw.(interface{ Params(string) string }); ok {
			return ctx.Params(key), true
		}
	case "Query":
		if ctx, ok := raw.(interface {
			Query(string, ...string) string
		}); ok {
			return ctx.Query(key, defaultValue...), true
		}
		if ctx, ok := raw.(interface{ Query(string) string }); ok {
			return ctx.Query(key), true
		}
	case "FormValue":
		if ctx, ok := raw.(interface{ FormValue(string) string }); ok {
			return ctx.FormValue(key), true
		}
	}

	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return "", false
	}

	// Use cached method if available
	cached := getCachedMethods(rv.Type())
	var method reflect.Value

	switch methodName {
	case "Query":
		if cached.query != nil {
			method = rv.Method(cached.query.Index)
		}
	case "Params":
		if cached.params != nil {
			method = rv.Method(cached.params.Index)
		}
	case "FormValue":
		if cached.formValue != nil {
			method = rv.Method(cached.formValue.Index)
		}
	}

	if !method.IsValid() {
		method = rv.MethodByName(methodName)
		if !method.IsValid() {
			return "", false
		}
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

	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return
	}

	// Use cached method if available
	cached := getCachedMethods(rv.Type())
	var method reflect.Value
	if cached.status != nil {
		method = rv.Method(cached.status.Index)
	} else {
		method = rv.MethodByName("Status")
		if !method.IsValid() {
			return
		}
	}

	if method.Type().NumIn() != 1 || method.Type().In(0).Kind() != reflect.Int {
		return
	}
	method.Call([]reflect.Value{reflect.ValueOf(status)})
}

func callJSONReflect(raw any, status int, data any) error {
	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return errNoJSONMethod
	}

	// Use cached method if available
	cached := getCachedMethods(rv.Type())
	var method reflect.Value
	if cached.json != nil {
		method = rv.Method(cached.json.Index)
	} else {
		method = rv.MethodByName("JSON")
		if !method.IsValid() {
			return errNoJSONMethod
		}
	}

	switch method.Type().NumIn() {
	case 1:
		results := method.Call([]reflect.Value{reflect.ValueOf(data)})
		return extractError(results)
	case 2:
		if method.Type().In(0).Kind() != reflect.Int {
			return errNoJSONMethod
		}
		results := method.Call([]reflect.Value{
			reflect.ValueOf(status),
			reflect.ValueOf(data),
		})
		return extractError(results)
	default:
		return errNoJSONMethod
	}
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

func callStringMethod(method reflect.Value, key string, defaultValue ...string) string {
	methodType := method.Type()
	args := []reflect.Value{reflect.ValueOf(key)}

	if methodType.IsVariadic() {
		if len(defaultValue) > 0 {
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
			return ""
		}
	}

	results := method.Call(args)
	if len(results) == 0 {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}
	if str, ok := results[0].Interface().(string); ok {
		if str == "" && len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return str
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
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
		entry := cached.(validatorCacheEntry)
		return entry.re, entry.err
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
