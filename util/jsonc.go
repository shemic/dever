package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type jsoncFileCacheEntry struct {
	size       int64
	modTime    int64
	normalized []byte
}

var jsoncFileCache ConcurrentMap[string, jsoncFileCacheEntry]

// NormalizeJSONC 将 JSONC 文本转换为标准 JSON，支持移除注释和尾随逗号。
func NormalizeJSONC(input []byte) ([]byte, error) {
	trimmed := bytes.TrimPrefix(input, []byte{0xEF, 0xBB, 0xBF})
	withoutComments, err := stripJSONCComments(trimmed)
	if err != nil {
		return nil, err
	}
	return stripJSONCTrailingCommas(withoutComments)
}

// UnmarshalJSONC 先将 JSONC 归一化，再按标准 JSON 反序列化。
func UnmarshalJSONC(input []byte, target any) error {
	normalized, err := NormalizeJSONC(input)
	if err != nil {
		return err
	}
	return UnmarshalNormalizedJSON(normalized, target)
}

// UnmarshalNormalizedJSON 假定输入已经是标准 JSON，直接反序列化。
func UnmarshalNormalizedJSON(input []byte, target any) error {
	if err := json.Unmarshal(input, target); err != nil {
		return err
	}
	return nil
}

// ReadJSONCFile 按顺序读取候选文件，返回首个存在文件的归一化 JSON 内容和实际路径。
func ReadJSONCFile(paths ...string) ([]byte, string, error) {
	for _, path := range paths {
		normalized, ok, err := readCachedJSONCFile(path)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			continue
		}
		return normalized, path, nil
	}
	return nil, "", os.ErrNotExist
}

func readCachedJSONCFile(path string) ([]byte, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			jsoncFileCache.Delete(path)
			return nil, false, nil
		}
		return nil, false, err
	}

	size := info.Size()
	modTime := info.ModTime().UnixNano()
	if cached, ok := jsoncFileCache.Load(path); ok {
		entry := cached
		if entry.size == size && entry.modTime == modTime {
			return entry.normalized, true, nil
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			jsoncFileCache.Delete(path)
			return nil, false, nil
		}
		return nil, false, err
	}

	normalized, err := NormalizeJSONC(content)
	if err != nil {
		return nil, false, err
	}

	jsoncFileCache.Store(path, jsoncFileCacheEntry{
		size:       size,
		modTime:    modTime,
		normalized: normalized,
	})
	return normalized, true, nil
}

func stripJSONCComments(input []byte) ([]byte, error) {
	output := make([]byte, 0, len(input))
	inString := false
	escaped := false

	for i := 0; i < len(input); i++ {
		current := input[i]

		if inString {
			output = append(output, current)
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '"' {
				inString = false
			}
			continue
		}

		if current == '"' {
			inString = true
			output = append(output, current)
			continue
		}

		if current != '/' || i+1 >= len(input) {
			output = append(output, current)
			continue
		}

		next := input[i+1]
		switch next {
		case '/':
			i++
			for i+1 < len(input) {
				i++
				if input[i] == '\n' {
					output = append(output, '\n')
					break
				}
				if input[i] == '\r' {
					output = append(output, '\r')
					if i+1 < len(input) && input[i+1] == '\n' {
						i++
						output = append(output, '\n')
					}
					break
				}
			}
		case '*':
			i++
			closed := false
			for i+1 < len(input) {
				i++
				if input[i] == '\n' || input[i] == '\r' {
					output = append(output, input[i])
				}
				if input[i] == '*' && i+1 < len(input) && input[i+1] == '/' {
					i++
					closed = true
					break
				}
			}
			if !closed {
				return nil, fmt.Errorf("jsonc 块注释未闭合")
			}
		default:
			output = append(output, current)
		}
	}

	if inString {
		return nil, fmt.Errorf("jsonc 字符串未闭合")
	}

	return output, nil
}

func stripJSONCTrailingCommas(input []byte) ([]byte, error) {
	output := make([]byte, 0, len(input))
	inString := false
	escaped := false

	for i := 0; i < len(input); i++ {
		current := input[i]

		if inString {
			output = append(output, current)
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '"' {
				inString = false
			}
			continue
		}

		if current == '"' {
			inString = true
			output = append(output, current)
			continue
		}

		if current == ',' {
			next := nextNonWhitespaceByte(input, i+1)
			if next == ']' || next == '}' {
				continue
			}
		}

		output = append(output, current)
	}

	if inString {
		return nil, fmt.Errorf("jsonc 字符串未闭合")
	}

	return output, nil
}

func nextNonWhitespaceByte(input []byte, start int) byte {
	for i := start; i < len(input); i++ {
		switch input[i] {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return input[i]
		}
	}
	return 0
}
