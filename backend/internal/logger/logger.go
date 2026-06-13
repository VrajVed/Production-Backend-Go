package logger

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/VrajVed/Production-Backend-Go/internal/config"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

type LoggerService struct {
	nrApp *newrelic.Application
}

func NewLoggerService(cfg *config.ObservabilityConfig) *LoggerService {
	service := &LoggerService{}

	if cfg.NewRelic.LicenseKey == "" {
		fmt.Println("New Relic License key not provided, skipping initialization")
		return service
	}

	var configOptions []newrelic.ConfigOption

	configOptions = append(configOptions,
		newrelic.ConfigAppName(cfg.ServiceName),
		newrelic.ConfigLicense(cfg.NewRelic.LicenseKey),
		newrelic.ConfigAppLogForwardingEnabled(cfg.NewRelic.AppLogForwardingEnabled),
		newrelic.ConfigDistributedTracerEnabled(cfg.NewRelic.DistributedTracingEnabled),
	)

	if cfg.NewRelic.DebugLogging {
		configOptions = append(configOptions, newrelic.ConfigDebugLogger(os.Stdout))
	}

	app, err := newrelic.NewApplication(configOptions...)

	if err != nil {
		fmt.Printf("Failed to initialize New Relic: %v\n", err)
		return service
	}

	service.nrApp = app
	fmt.Printf("new Relic initialized for app: %s\n", cfg.ServiceName)
	return service
}

func (ls *LoggerService) Shutdown() {
	if ls.nrApp != nil {
		ls.nrApp.Shutdown(10 * time.Second)
	}
}

// NewLoggerWithService creates a logger with full config and logger service
func NewLoggerWithService(cfg *config.ObservabilityConfig, loggerService *LoggerService) zerolog.Logger {
	var logLevel zerolog.Level
	level := cfg.GetLogLevel()

	switch level {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	// Don't set global level - let each logger have its own level
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05"
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	var writer io.Writer

	// Setup base writer
	var baseWriter io.Writer
	if cfg.IsProduction() && cfg.Logging.Format == "json" {
		// In production, write to stdout
		baseWriter = os.Stdout

		// Wrap with New Relic zerologWriter for log forwarding in production
		if loggerService != nil && loggerService.nrApp != nil {
			nrWriter := zerologWriter.New(baseWriter, loggerService.nrApp)
			writer = nrWriter
		} else {
			writer = baseWriter
		}
	} else {
		// Development mode - use console writer
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "2006-01-02 15:04:05"}
		writer = consoleWriter
	}

	// Note: New Relic log forwarding is now handled automatically by zerologWriter integration

	logger := zerolog.New(writer).
		Level(logLevel).
		With().
		Timestamp().
		Str("service", cfg.ServiceName).
		Str("environment", cfg.Environment).
		Logger()

	// Include stack traces for errors in development
	if !cfg.IsProduction() {
		logger = logger.With().Stack().Logger()
	}

	return logger
}

func NewLogger(level string, isProd bool) zerolog.Logger {
	return NewLoggerWithService(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{
			Level: level,
		},
		Environment: func() string {
			if isProd {
				return "production"
			}
			return "development"
		}(),
	}, nil)
}

func NewLoggerWithConfig(cfg *config.ObservabilityConfig) zerolog.Logger {
	return NewLoggerWithService(cfg, nil)
}
