// Package ztrace provides distributed tracing parse/propagation helpers compatible with W3C Trace Context / OpenTelemetry.
// Package ztrace 提供分布式链路追踪的解析与透传工具，兼容 W3C Trace Context / OpenTelemetry。
// TraceID/SpanID use numeric types for allocation-free hot paths.
// TraceID/SpanID 使用数字类型，热路径零分配。
package ztrace

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/aiyang-zh/zhenyi-base/zid"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"go.uber.org/zap"
)

// TraceparentHeader is the HTTP header name for W3C Trace Context `traceparent`.
// TraceparentHeader 是 W3C Trace Context 的 HTTP 头名称 `traceparent`。
const TraceparentHeader = "traceparent"

// ParseTraceparent parses W3C traceparent format "00-{traceId}-{spanId}-{flags}".
// ParseTraceparent 解析 W3C traceparent 格式 "00-{traceId}-{spanId}-{flags}"。
// It returns (traceID [2]uint64, spanID uint64); invalid format returns zero values.
// 返回 (traceID [2]uint64, spanID uint64)。格式非法时返回零值。
func ParseTraceparent(s string) (traceID [2]uint64, spanID uint64) {
	if s == "" {
		return [2]uint64{}, 0
	}
	parts := strings.SplitN(s, "-", 5)
	if len(parts) < 4 {
		return [2]uint64{}, 0
	}
	tidStr := strings.TrimSpace(parts[1])
	sidStr := strings.TrimSpace(parts[2])
	if len(tidStr) != 32 || len(sidStr) != 16 {
		return [2]uint64{}, 0
	}
	tidBuf, err := hex.DecodeString(tidStr)
	if err != nil || len(tidBuf) != 16 {
		return [2]uint64{}, 0
	}
	traceID[0] = binary.BigEndian.Uint64(tidBuf[0:8])
	traceID[1] = binary.BigEndian.Uint64(tidBuf[8:16])

	sidBuf, err := hex.DecodeString(sidStr)
	if err != nil || len(sidBuf) != 8 {
		return [2]uint64{}, 0
	}
	spanID = binary.BigEndian.Uint64(sidBuf)
	return traceID, spanID
}

// ParseTraceparentString parses traceparent and returns hex strings.
// ParseTraceparentString 解析 hex 字符串并返回字符串形式。建议优先使用 ParseTraceparent。
func ParseTraceparentString(s string) (traceIDStr, spanIDStr string) {
	traceID, spanID := ParseTraceparent(s)
	return TraceIDToHex(traceID), SpanIDToHex(spanID)
}

// TraceIDToHex converts TraceID to 32-char hex string (for serialization/logging).
// TraceIDToHex 将 TraceID 转为 32 位 hex 字符串（序列化、日志用）。
func TraceIDToHex(t [2]uint64) string {
	if t[0] == 0 && t[1] == 0 {
		return ""
	}
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], t[0])
	binary.BigEndian.PutUint64(buf[8:16], t[1])
	return hex.EncodeToString(buf[:])
}

// SpanIDToHex converts SpanID to 16-char hex string (for serialization/logging).
// SpanIDToHex 将 SpanID 转为 16 位 hex 字符串（序列化、日志用）。
func SpanIDToHex(s uint64) string {
	if s == 0 {
		return ""
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], s)
	return hex.EncodeToString(buf[:])
}

// TraceIDIsZero checks whether TraceID is zero.
// TraceIDIsZero 判断 TraceID 是否为零。
func TraceIDIsZero(t [2]uint64) bool {
	return t[0] == 0 && t[1] == 0
}

// GenerateTraceID generates a new 128-bit TraceID compliant with W3C/OTel.
// GenerateTraceID 生成新的 128-bit TraceID，符合 W3C/OTel 规范。crypto/rand 保证全局唯一性。
func GenerateTraceID() [2]uint64 {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return [2]uint64{}
	}
	return [2]uint64{
		binary.BigEndian.Uint64(buf[0:8]),
		binary.BigEndian.Uint64(buf[8:16]),
	}
}

// GenerateSpanID generates a new 64-bit SpanID via zid.NextFast (~5ns).
// GenerateSpanID 生成新的 64-bit SpanID，委托 zid.NextFast（~5ns）。需在 main 中先调用 zid.InitFast(nodeId)。
func GenerateSpanID() uint64 {
	return zid.NextFast()
}

// ParseOrGenerate parses traceparent, generating new trace when missing/invalid.
// ParseOrGenerate 解析 traceparent，若无或无效则生成新 trace。便于 TraceExtractor 一键注入。
func ParseOrGenerate(traceparent string) (traceID [2]uint64, spanID uint64) {
	traceID, spanID = ParseTraceparent(traceparent)
	if TraceIDIsZero(traceID) && spanID == 0 {
		return GenerateTraceID(), GenerateSpanID()
	}
	return traceID, spanID
}

// ExtractOrGenerateFromHTTPRequest extracts trace from http.Request or generates one.
// ExtractOrGenerateFromHTTPRequest 从 http.Request 提取 trace，若无则生成。适用于 Gin 等 HTTP 根入口。
func ExtractOrGenerateFromHTTPRequest(r *http.Request) (traceID [2]uint64, spanID uint64) {
	traceID, spanID = ExtractFromHTTPRequest(r)
	if TraceIDIsZero(traceID) && spanID == 0 {
		return GenerateTraceID(), GenerateSpanID()
	}
	return traceID, spanID
}

// TraceIDFromHex parses TraceID from 32-char hex string, returning zero on failure.
// TraceIDFromHex 从 32 位 hex 字符串解析 TraceID，失败返回零值。
func TraceIDFromHex(s string) [2]uint64 {
	if len(s) != 32 {
		return [2]uint64{}
	}
	buf, err := hex.DecodeString(s)
	if err != nil || len(buf) != 16 {
		return [2]uint64{}
	}
	return [2]uint64{
		binary.BigEndian.Uint64(buf[0:8]),
		binary.BigEndian.Uint64(buf[8:16]),
	}
}

// SpanIDFromHex parses SpanID from 16-char hex string, returning 0 on failure.
// SpanIDFromHex 从 16 位 hex 字符串解析 SpanID，失败返回 0。
func SpanIDFromHex(s string) uint64 {
	if len(s) != 16 {
		return 0
	}
	buf, err := hex.DecodeString(s)
	if err != nil || len(buf) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(buf)
}

// ExtractFromHTTPRequest parses TraceID/SpanID from request traceparent header.
// ExtractFromHTTPRequest 从 http.Request 的 traceparent 头解析 TraceID/SpanID。
func ExtractFromHTTPRequest(r *http.Request) (traceID [2]uint64, spanID uint64) {
	if r == nil {
		return [2]uint64{}, 0
	}
	return ParseTraceparent(r.Header.Get(TraceparentHeader))
}

// LogFields returns zap fields for trace logging; nil when both traceID/spanID are zero.
// LogFields 返回可追加到日志的 trace 字段。traceID 为零且 spanID 为 0 时返回 nil。
func LogFields(traceID [2]uint64, spanID uint64) []zap.Field {
	if TraceIDIsZero(traceID) && spanID == 0 {
		return nil
	}
	tidHex := TraceIDToHex(traceID)
	sidHex := SpanIDToHex(spanID)
	if sidHex == "" {
		return []zap.Field{zlog.FastString("traceId", tidHex)}
	}
	if tidHex == "" {
		return []zap.Field{zlog.FastString("spanId", sidHex)}
	}
	return []zap.Field{
		zlog.FastString("traceId", tidHex),
		zlog.FastString("spanId", sidHex),
	}
}
