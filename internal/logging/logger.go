package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Logger struct {
	json bool
	l    *log.Logger
}

type Field struct {
	Key string
	Val any
}

func New(jsonEnabled bool) *Logger {
	return &Logger{
		json: jsonEnabled,
		l:    log.New(os.Stdout, "", 0),
	}
}

func (lg *Logger) SetJSON(enabled bool) {
	lg.json = enabled
}

func (lg *Logger) Info(msg string, fields ...Field) {
	lg.print("info", msg, fields...)
}

func (lg *Logger) Warn(msg string, fields ...Field) {
	lg.print("warn", msg, fields...)
}

func (lg *Logger) Error(msg string, fields ...Field) {
	lg.print("error", msg, fields...)
}

func (lg *Logger) print(level, msg string, fields ...Field) {
	if lg.json {
		payload := map[string]any{
			"ts":    time.Now().Format(time.RFC3339),
			"level": level,
			"msg":   msg,
		}
		for _, f := range fields {
			payload[f.Key] = f.Val
		}
		b, _ := json.Marshal(payload)
		lg.l.Println(string(b))
		return
	}
	parts := []string{time.Now().Format(time.RFC3339), strings.ToUpper(level), msg}
	for _, f := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", f.Key, f.Val))
	}
	lg.l.Println(strings.Join(parts, " "))
}
