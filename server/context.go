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
)

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
	Raw any
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
	if jErr := c.JSONWithStatus(status, payload); jErr != nil {
		return jErr
	}
	return nil
}

func normalizePayload(status int, data any) any {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		if data == nil {
			return map[string]any{
				"code":   status,
				"status": 2,
				"msg":    http.StatusText(status),
				"data":   nil,
			}
		}
		return data
	}

	if data == nil {
		return map[string]any{
			"code":   status,
			"status": 1,
			"msg":    "success",
			"data":   nil,
		}
	}

	if envelope, ok := data.(map[string]any); ok {
		if looksLikeEnvelope(envelope) {
			envelopeCopy := cloneMap(envelope)
			if _, exists := envelopeCopy["code"]; !exists {
				envelopeCopy["code"] = status
			}
			if _, exists := envelopeCopy["status"]; !exists {
				envelopeCopy["status"] = 1
			}
			if _, exists := envelopeCopy["msg"]; !exists {
				envelopeCopy["msg"] = "success"
			}
			if _, exists := envelopeCopy["data"]; !exists {
				envelopeCopy["data"] = nil
			}
			return envelopeCopy
		}
		return map[string]any{
			"code":   status,
			"status": 1,
			"msg":    "success",
			"data":   envelope,
		}
	}

	return map[string]any{
		"code":   status,
		"status": 1,
		"msg":    "success",
		"data":   data,
	}
}

func normalizeErrorPayload(status int, err error) map[string]any {
	msg := http.StatusText(status)
	if err != nil && err.Error() != "" {
		msg = err.Error()
	}
	return map[string]any{
		"code":   status,
		"status": 2,
		"msg":    msg,
		"data":   nil,
	}
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func looksLikeEnvelope(m map[string]any) bool {
	if m == nil {
		return false
	}
	if _, ok := m["code"]; ok {
		return true
	}
	if _, ok := m["status"]; ok {
		return true
	}
	if _, ok := m["msg"]; ok {
		return true
	}
	if _, ok := m["data"]; ok {
		return true
	}
	return false
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
		value = defaultVal
	}

	rule = strings.TrimSpace(rule)
	if rule != "" && value != "" {
		re, err := compileValidator(rule)
		if err != nil {
			c.abort(fmt.Errorf("%s 校验规则错误: %v", desc, err))
		} else if !re.MatchString(value) {
			c.abort(fmt.Errorf("%s 格式不正确", desc))
		}
	}

	return value
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

	method := rv.MethodByName(methodName)
	if !method.IsValid() {
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

	rv := reflect.ValueOf(raw)
	if !rv.IsValid() {
		return
	}
	method := rv.MethodByName("Status")
	if !method.IsValid() {
		return
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

	method := rv.MethodByName("JSON")
	if !method.IsValid() {
		return errNoJSONMethod
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
