package logengine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

var (
	TimestampFields = []string{"ts", "time", "timestamp"}
	LevelFields     = []string{"level", "lvl"}
	ModuleFields    = []string{"module", "logger"}
	HeightFields    = []string{"height"}
)

const (
	LevelDebug uint16 = 1 << iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel parses log level strings from actual log output, accepting both
// canonical names and common abbreviations (dbg, wrn, err, fatal, panic).
func ParseLevel(s string) uint16 {
	switch strings.ToLower(s) {
	case "debug", "dbg":
		return LevelDebug
	case "info", "inf":
		return LevelInfo
	case "warn", "warning", "wrn":
		return LevelWarn
	case "error", "err", "fatal", "panic":
		return LevelError
	default:
		return 0
	}
}

// ParseLevelName parses canonical level names from user input (debug, info,
// warn, error). Unlike ParseLevel, it does not accept abbreviations or
// severity aliases like fatal/panic.
func ParseLevelName(s string) uint16 {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return 0
	}
}

func ExtractTimestampNano(line []byte) int64 {
	for _, field := range TimestampFields {
		r := gjson.GetBytes(line, field)
		if r.Exists() {
			if t, err := time.Parse(time.RFC3339Nano, r.Str); err == nil {
				return t.UnixNano()
			}
		}
	}
	return 0
}

func ExtractTimestampStr(line []byte) string {
	for _, field := range TimestampFields {
		r := gjson.GetBytes(line, field)
		if r.Exists() {
			return r.Str
		}
	}
	return ""
}

func ExtractLevel(line []byte) string {
	for _, field := range LevelFields {
		r := gjson.GetBytes(line, field)
		if r.Exists() {
			return strings.ToLower(r.Str)
		}
	}
	return ""
}

func ExtractModule(line []byte) string {
	for _, f := range ModuleFields {
		v := gjson.GetBytes(line, f)
		if v.Exists() {
			return v.String()
		}
	}
	return ""
}

func ExtractHeight(line []byte) int64 {
	for _, f := range HeightFields {
		v := gjson.GetBytes(line, f)
		if v.Exists() {
			return v.Int()
		}
	}
	return 0
}

func HasLevelOrAbove(flags, level uint16) bool {
	mask := uint16(0)
	for l := level; l <= LevelError; l <<= 1 {
		mask |= l
	}
	return flags&mask != 0
}

// ParseTimestamp parses an RFC3339 or numeric nanosecond timestamp string.
func ParseTimestamp(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp %q: not a number or RFC3339", s)
	}
	return t.UnixNano(), nil
}
