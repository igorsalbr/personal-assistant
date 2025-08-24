package log

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"
)

// ContextKey represents a key for context values
type ContextKey string

const (
	// RequestIDKey is the context key for request IDs
	RequestIDKey ContextKey = "request_id"
	// TenantIDKey is the context key for tenant IDs
	TenantIDKey ContextKey = "tenant_id"
	// UserIDKey is the context key for user IDs
	UserIDKey ContextKey = "user_id"
)

// Logger wraps zerolog.Logger with additional functionality
type Logger struct {
	zerolog.Logger
}

// Init initializes the logging system
func Init(level string) *Logger {
	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Set log level
	logLevel := zerolog.InfoLevel
	switch strings.ToLower(level) {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn", "warning":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	case "fatal":
		logLevel = zerolog.FatalLevel
	case "panic":
		logLevel = zerolog.PanicLevel
	case "disabled":
		logLevel = zerolog.Disabled
	}

	// Configure output
	var output zerolog.ConsoleWriter
	if isDevMode() {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
		}
		zerolog.SetGlobalLevel(logLevel)
		log.Logger = log.Output(output)
	} else {
		zerolog.SetGlobalLevel(logLevel)
	}

	return &Logger{Logger: log.Logger}
}

// WithContext returns a logger with context information
func (l *Logger) WithContext(ctx context.Context) *Logger {
	logger := l.Logger

	// Add request ID if present
	if reqID := ctx.Value(RequestIDKey); reqID != nil {
		logger = logger.With().Str("request_id", reqID.(string)).Logger()
	}

	// Add tenant ID if present
	if tenantID := ctx.Value(TenantIDKey); tenantID != nil {
		logger = logger.With().Str("tenant_id", tenantID.(string)).Logger()
	}

	// Add user ID if present
	if userID := ctx.Value(UserIDKey); userID != nil {
		logger = logger.With().Str("user_id", userID.(string)).Logger()
	}

	// Add trace information if available
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		logger = logger.With().
			Str("trace_id", span.SpanContext().TraceID().String()).
			Str("span_id", span.SpanContext().SpanID().String()).
			Logger()
	}

	return &Logger{Logger: logger}
}

// WithTenant adds tenant information to the logger
func (l *Logger) WithTenant(tenantID string) *Logger {
	return &Logger{Logger: l.With().Str("tenant_id", tenantID).Logger()}
}

// WithUser adds user information to the logger
func (l *Logger) WithUser(userID string) *Logger {
	return &Logger{Logger: l.With().Str("user_id", userID).Logger()}
}

// WithRequest adds request information to the logger
func (l *Logger) WithRequest(requestID string) *Logger {
	return &Logger{Logger: l.With().Str("request_id", requestID).Logger()}
}

// WithComponent adds component information to the logger
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{Logger: l.With().Str("component", component).Logger()}
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	logger := l.Logger
	for k, v := range fields {
		logger = logger.With().Interface(k, v).Logger()
	}
	return &Logger{Logger: logger}
}

// LogTokenUsage logs token usage information
func (l *Logger) LogTokenUsage(component string, promptTokens, completionTokens, totalTokens int) {
	l.Info().
		Str("component", component).
		Int("prompt_tokens", promptTokens).
		Int("completion_tokens", completionTokens).
		Int("total_tokens", totalTokens).
		Msg("token usage")
}

// LogToolCall logs a tool call
func (l *Logger) LogToolCall(toolName string, input interface{}, duration time.Duration) {
	l.Info().
		Str("tool_name", toolName).
		Interface("input", input).
		Dur("duration", duration).
		Msg("tool call")
}

// LogAPICall logs an external API call
func (l *Logger) LogAPICall(service, method, endpoint string, statusCode int, duration time.Duration) {
	event := l.Info().
		Str("service", service).
		Str("method", method).
		Str("endpoint", endpoint).
		Int("status_code", statusCode).
		Dur("duration", duration)

	if statusCode >= 400 {
		event = l.Warn().
			Str("service", service).
			Str("method", method).
			Str("endpoint", endpoint).
			Int("status_code", statusCode).
			Dur("duration", duration)
	}

	event.Msg("api call")
}

// LogMessageProcessing logs message processing information
func (l *Logger) LogMessageProcessing(messageID, from, to string, messageType string, duration time.Duration) {
	l.Info().
		Str("message_id", messageID).
		Str("from", from).
		Str("to", to).
		Str("message_type", messageType).
		Dur("duration", duration).
		Msg("message processed")
}

// SanitizeText removes or masks sensitive information from text
func SanitizeText(text string) string {
	// In a real implementation, you might want to:
	// - Replace phone numbers with XXX-XXXX
	// - Replace email addresses
	// - Replace other PII patterns
	if len(text) > 100 {
		return text[:97] + "..."
	}
	return text
}

// isDevMode checks if we're in development mode
func isDevMode() bool {
	env := strings.ToLower(os.Getenv("GO_ENV"))
	return env == "development" || env == "dev" || env == ""
}

// Global logger instance
var GlobalLogger *Logger

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	GlobalLogger = logger
}

// FromContext returns a logger with context information
func FromContext(ctx context.Context) *Logger {
	if GlobalLogger == nil {
		GlobalLogger = Init("info")
	}
	return GlobalLogger.WithContext(ctx)
}

// Info logs an info message
func Info() *zerolog.Event {
	if GlobalLogger == nil {
		GlobalLogger = Init("info")
	}
	return GlobalLogger.Info()
}

// Error logs an error message
func Error() *zerolog.Event {
	if GlobalLogger == nil {
		GlobalLogger = Init("info")
	}
	return GlobalLogger.Error()
}

// Debug logs a debug message
func Debug() *zerolog.Event {
	if GlobalLogger == nil {
		GlobalLogger = Init("info")
	}
	return GlobalLogger.Debug()
}

// Warn logs a warning message
func Warn() *zerolog.Event {
	if GlobalLogger == nil {
		GlobalLogger = Init("info")
	}
	return GlobalLogger.Warn()
}
