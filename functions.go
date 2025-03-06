package bob

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var (
	loggerInitLock         sync.RWMutex
	tracerInitLock         sync.RWMutex
	zapConfig              *zap.Config
	singletonTraceProvider trace.TracerProvider
)

func Initialize(c *Config) error {
	var err error

	if singletonTraceProvider, err = InitializeTraceProvider(c.TracerConfig); err != nil {
		return fmt.Errorf("problem initializing tracing: %w", err)
	}

	if err = InitializeGlobalLogger(c.LoggerConfig); err != nil {
		return fmt.Errorf("problem initializing logging: %w", err)
	}

	return nil
}

// InitializeGlobalLogger defines global zap and open-telemetry zap loggers configured via the given monitor.Config.
func InitializeGlobalLogger(c *LoggerConfig) error {
	zapConfig, err := getZapConfig(c)
	if err != nil {
		return fmt.Errorf("problem creating zap configuration: %w", err)
	}

	zapLogger := zap.Must(zapConfig.Build())
	zapLogger = zapLogger.With(zap.String("id", c.AppID))

	defer func() {
		_ = zapLogger.Sync()
	}()

	otelLogger := otelzap.New(zapLogger, otelzap.WithMinLevel(zapcore.InfoLevel))

	defer func() {
		_ = otelLogger.Sync()
	}()

	zap.ReplaceGlobals(zapLogger)
	otelzap.ReplaceGlobals(otelLogger)

	return nil
}

// InitializeTraceProvider creates an open-telemetry trace provider configured via the given monitor.Config.
func InitializeTraceProvider(c *TracerConfig) (trace.TracerProvider, error) {
	providerResource, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(c.AppName),
			semconv.ServiceInstanceIDKey.String(c.AppID),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("problem creating tracer provider resources: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(providerResource)}

	if len(c.OtelExporters) < 1 {
		return noop.NewTracerProvider(), nil
	}

	for _, exporter := range c.OtelExporters {
		switch exporter {
		case "console":
			consoleExporter, err := newConsoleExporter(c)
			if err != nil {
				return nil, fmt.Errorf("problem creating tracer console exporter: %w", err)
			}

			opts = append(opts, sdktrace.WithBatcher(consoleExporter))
		case "otlp":
			otlpExporter, err := newOtlpExporter(c)
			if err != nil {
				return nil, fmt.Errorf("problem creating tracer otlp exporter: %w", err)
			}

			opts = append(opts, sdktrace.WithBatcher(otlpExporter))
		default:
			return nil, fmt.Errorf("unsupported exporter found: %s", exporter)
		}
	}

	return sdktrace.NewTracerProvider(opts...), nil
}

// ShutdownTraceProvider ...
func ShutdownTraceProvider(ctx context.Context, traceProvider trace.TracerProvider) error {
	if sdkTraceProvider, ok := traceProvider.(*sdktrace.TracerProvider); ok {
		_ = sdkTraceProvider.ForceFlush(ctx)

		if err := sdkTraceProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("problem shutting down trace provider: %w", err)
		}

		return nil
	}

	return nil
}

// NewLogger creates an open-telemetry zap logger with the given name, and attaches info+ logs to traces.
func NewLogger(name string) *otelzap.Logger {
	return otelzap.New(zap.L().Named(name), otelzap.WithMinLevel(zapcore.InfoLevel))
}

// NewObserverLogger creates an open-telemetry zap logger with the given name, and provides a struct that can be
// utilized to observe log messages created with the provided logger.
func NewObserverLogger(name string) (*otelzap.Logger, *observer.ObservedLogs) {
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)

	return otelzap.New(observedLogger, otelzap.WithMinLevel(zapcore.InfoLevel)),
		observedLogs
}

// NewNoopTracer creates a no-op tracer with the given name.
func NewNoopTracer(tracerName string) trace.Tracer {
	return noop.NewTracerProvider().Tracer(tracerName)
}

// RecordError is a helper function that attaches an error to a span.
func RecordError(span trace.Span, err error) {
	if span == nil || !span.IsRecording() {
		return
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetGlobalLogLevel updates the log level for log messages throughout the application.
func SetGlobalLogLevel(level string) error {
	zapLogLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("problem parsing log level: %w", err)
	}

	if zapConfig == nil {
		return fmt.Errorf("global logger not initialized")
	}

	zapConfig.Level.SetLevel(zapLogLevel)

	return nil
}

// -- Private functions --

func newConsoleExporter(c *TracerConfig) (sdktrace.SpanExporter, error) {
	if c.OtelConsoleFormat == "production" {
		return stdouttrace.New(
			stdouttrace.WithWriter(os.Stdout),
		)
	}

	return stdouttrace.New(
		stdouttrace.WithWriter(os.Stdout),
		stdouttrace.WithPrettyPrint(),
	)
}

func newOtlpExporter(c *TracerConfig) (sdktrace.SpanExporter, error) {
	// NOTE: default http port is 4318, default grpc port is 4317
	var exporter sdktrace.SpanExporter
	var err error

	switch c.OtlpEndpointKind {
	case "http":
		exporter, err = otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(fmt.Sprintf("%s:%d", c.OtlpHost, c.OtlpPort)),
		)
	case "grpc":
		exporter, err = otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithEndpoint(fmt.Sprintf("%s:%d", c.OtlpHost, c.OtlpPort)),
		)
	default:
		return nil, fmt.Errorf("unsupported otlp endpoint kind: %s", c.OtlpEndpointKind)
	}

	if err != nil {
		return nil, fmt.Errorf("problem creating otlp exporter: %w", err)
	}

	return exporter, nil
}

func getZapConfig(c *LoggerConfig) (*zap.Config, error) {
	if zapConfig != nil {
		return zapConfig, nil
	}

	var newZapConfig zap.Config

	// Parse out the logging configuration
	switch c.LogConfig {
	case "production":
		newZapConfig = zap.NewProductionConfig()
	case "development":
		newZapConfig = zap.NewDevelopmentConfig()
	default:
		return nil, fmt.Errorf("unsupported log config value: '%s'", c.LogConfig)
	}

	// Parse out the logging level
	if c.LogLevel != "" {
		var err error

		newZapConfig.Level, err = zap.ParseAtomicLevel(c.LogLevel)
		if err != nil {
			return nil, fmt.Errorf("unsupported log level value: '%s'", c.LogLevel)
		}
	}

	// Parse out the logging format / encoding
	if c.LogFormat != "" && c.LogFormat != "console" && c.LogFormat != "json" {
		return nil, fmt.Errorf("unsupported log format value: '%s'", c.LogFormat)
	}

	newZapConfig.Encoding = c.LogFormat
	newZapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Handle color for console encoding
	if newZapConfig.Encoding == "console" && c.LogColor {
		newZapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	zapConfig = &newZapConfig

	return &newZapConfig, nil
}
