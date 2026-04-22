package dlog

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/shemic/dever/util"
)

type Fields map[string]any

func AccessFields(event, message string, fields Fields) {
	writeFields(accessOut, "INFO", event, message, fields)
}

func ErrorFields(event, message string, fields Fields) {
	writeFields(errorOut, "ERROR", event, message, fields)
}

func writeFields(writer io.Writer, level, event, message string, fields Fields) {
	if writer == nil {
		return
	}

	entry := map[string]any{
		"level":   strings.ToUpper(strings.TrimSpace(level)),
		"time":    time.Now().UTC().Format(time.RFC3339Nano),
		"event":   strings.TrimSpace(event),
		"message": strings.TrimSpace(message),
	}

	for key, value := range fields {
		fieldKey := strings.TrimSpace(key)
		if fieldKey == "" || value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		entry[fieldKey] = value
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	writerMu.Lock()
	defer writerMu.Unlock()
	_, _ = writer.Write(append(data, '\n'))
}

func ErrorValue(err error) any {
	if err == nil {
		return nil
	}
	return util.ToStringTrimmed(err.Error())
}
