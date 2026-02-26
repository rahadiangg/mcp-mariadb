package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rahadiangg/mcp-mariadb/internal/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds all configuration for the MariaDB MCP server
type Config struct {
	// Database connection
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Charset  string

	// SSL
	SSL                bool
	SSLCA              string
	SSLCert            string
	SSLKey             string
	SSLVerifyCert      bool
	SSLVerifyIdentity  bool

	// Server behavior
	ReadOnly     bool
	MaxPoolSize  int
	PoolRecycle  int // seconds

	// Security limits
	MaxQuerySize int // bytes

	// Connection timeouts (seconds)
	ConnMaxIdleTime int
	ConnTimeout     int
	ReadTimeout     int
	WriteTimeout    int

	// CORS configuration
	CORSAllowOrigins     string
	CORSAllowMethods     string
	CORSAllowHeaders     string
	CORSAllowCredentials bool

	// Embeddings
	EmbeddingProvider string // "openai", "gemini", or "" for disabled
	OpenAIKey         string
	GeminiKey         string
}

// Load reads configuration from environment variables (with MCP_MARIADB_ prefix)
func Load() (*Config, *zap.Logger, error) {
	// Load .env file if present
	_ = godotenv.Load()

	// Initialize logger first
	logger, err := initLogger()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	getEnv := func(key, defaultVal string) string {
		val := os.Getenv("MCP_MARIADB_" + key)
		if val == "" {
			val = os.Getenv(key) // fallback to non-prefixed version
		}
		if val == "" {
			return defaultVal
		}
		return val
	}

	getEnvInt := func(key string, defaultVal int) int {
		val := getEnv(key, "")
		if val == "" {
			return defaultVal
		}
		i, err := strconv.Atoi(val)
		if err != nil {
			logger.Warn("Invalid integer value for config key, using default",
				zap.String("key", key),
				zap.String("value", val),
				zap.Int("default", defaultVal))
			return defaultVal
		}
		return i
	}

	getEnvBool := func(key string, defaultVal bool) bool {
		val := getEnv(key, "")
		if val == "" {
			return defaultVal
		}
		b, err := strconv.ParseBool(val)
		if err != nil {
			logger.Warn("Invalid boolean value for config key, using default",
				zap.String("key", key),
				zap.String("value", val),
				zap.Bool("default", defaultVal))
			return defaultVal
		}
		return b
	}

	cfg := &Config{
		Host:     getEnv("HOST", "localhost"),
		Port:     getEnvInt("PORT", 3306),
		User:     getEnv("USER", ""),
		Password: getEnv("PASSWORD", ""),
		Database: getEnv("DATABASE", ""),
		Charset:  getEnv("CHARSET", ""),
		SSL:      getEnvBool("SSL", false),
		SSLCA:    getEnv("SSL_CA", ""),
		SSLCert:  getEnv("SSL_CERT", ""),
		SSLKey:   getEnv("SSL_KEY", ""),
		SSLVerifyCert:     getEnvBool("SSL_VERIFY_CERT", true),
		SSLVerifyIdentity: getEnvBool("SSL_VERIFY_IDENTITY", false),
		ReadOnly:          getEnvBool("READ_ONLY", true),
		MaxPoolSize:       getEnvInt("MAX_POOL_SIZE", 10),
		PoolRecycle:       getEnvInt("POOL_RECYCLE", 3600),
		MaxQuerySize:      getEnvInt("MAX_QUERY_SIZE", 1048576), // 1MB default
		ConnMaxIdleTime:   getEnvInt("CONN_MAX_IDLETIME", 600), // 10 minutes
		ConnTimeout:       getEnvInt("CONN_TIMEOUT", 10),        // 10 seconds
		ReadTimeout:       getEnvInt("READ_TIMEOUT", 30),        // 30 seconds
		WriteTimeout:      getEnvInt("WRITE_TIMEOUT", 30),       // 30 seconds
		CORSAllowOrigins:  getEnv("CORS_ALLOW_ORIGINS", ""),
		CORSAllowMethods:  getEnv("CORS_ALLOW_METHODS", "GET, POST, OPTIONS"),
		CORSAllowHeaders:  getEnv("CORS_ALLOW_HEADERS", "Content-Type, Authorization"),
		CORSAllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", false),
		EmbeddingProvider: strings.ToLower(getEnv("EMBEDDING_PROVIDER", "")),
		OpenAIKey:         getEnv("OPENAI_KEY", ""),
		GeminiKey:         getEnv("GEMINI_KEY", ""),
	}

	// Validate required settings
	if cfg.User == "" {
		logger.Error("DB_USER (MCP_MARIADB_USER) is required")
		return nil, logger, fmt.Errorf("DB_USER is required")
	}

	// Validate port range
	if err := util.ValidatePort(cfg.Port); err != nil {
		logger.Error("Invalid port", zap.Error(err))
		return nil, logger, err
	}

	// Validate SSL files if SSL is enabled
	if cfg.SSL {
		if cfg.SSLCA != "" {
			if err := util.ValidateSSLFile(cfg.SSLCA, false); err != nil {
				logger.Warn("SSL CA file validation warning", zap.Error(err))
				// Continue on warnings for non-key files
			}
		}
		if cfg.SSLCert != "" {
			if err := util.ValidateSSLFile(cfg.SSLCert, false); err != nil {
				logger.Warn("SSL certificate file validation warning", zap.Error(err))
			}
		}
		if cfg.SSLKey != "" {
			if err := util.ValidateSSLFile(cfg.SSLKey, true); err != nil {
				logger.Error("SSL key file validation failed", zap.Error(err))
				return nil, logger, err
			}
		}
	}

	// Validate CORS configuration
	if cfg.CORSAllowOrigins == "*" {
		logger.Warn("CORS_ALLOW_ORIGINS set to wildcard (*) - allows requests from any origin")
	}

	// Validate embedding configuration
	if cfg.EmbeddingProvider != "" {
		switch cfg.EmbeddingProvider {
		case "openai":
			if cfg.OpenAIKey == "" {
				logger.Error("OPENAI_KEY is required when EMBEDDING_PROVIDER is 'openai'")
				return nil, logger, fmt.Errorf("OPENAI_KEY is required for openai provider")
			}
			if err := util.ValidateAPIKeyFormat("openai", cfg.OpenAIKey); err != nil {
				logger.Error("Invalid OPENAI_KEY format", zap.Error(err))
				return nil, logger, fmt.Errorf("invalid OPENAI_KEY: %w", err)
			}
		case "gemini":
			if cfg.GeminiKey == "" {
				logger.Error("GEMINI_KEY is required when EMBEDDING_PROVIDER is 'gemini'")
				return nil, logger, fmt.Errorf("GEMINI_KEY is required for gemini provider")
			}
			if err := util.ValidateAPIKeyFormat("gemini", cfg.GeminiKey); err != nil {
				logger.Error("Invalid GEMINI_KEY format", zap.Error(err))
				return nil, logger, fmt.Errorf("invalid GEMINI_KEY: %w", err)
			}
		default:
			logger.Warn("Unknown embedding provider, disabling embeddings",
				zap.String("provider", cfg.EmbeddingProvider))
			cfg.EmbeddingProvider = ""
		}
	}

	logger.Info("Configuration loaded",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("user", cfg.User),
		zap.String("database", cfg.Database),
		zap.Bool("ssl", cfg.SSL),
		zap.Bool("read_only", cfg.ReadOnly),
		zap.Int("max_pool_size", cfg.MaxPoolSize),
		zap.Int("max_query_size", cfg.MaxQuerySize),
		zap.Int("conn_max_idle_time", cfg.ConnMaxIdleTime),
		zap.Int("conn_timeout", cfg.ConnTimeout),
		zap.Int("read_timeout", cfg.ReadTimeout),
		zap.Int("write_timeout", cfg.WriteTimeout),
		zap.String("cors_allow_origins", cfg.CORSAllowOrigins),
		zap.String("embedding_provider", cfg.EmbeddingProvider),
	)

	return cfg, logger, nil
}

// initLogger creates a zap logger with console and optional file output
func initLogger() (*zap.Logger, error) {
	// Get log level from env
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}

	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Get log file path (optional)
	logFile := os.Getenv("LOG_FILE")

	// Console encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Build cores
	var cores []zapcore.Core

	// Console core
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)
	cores = append(cores, consoleCore)

	// File core (if specified)
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(file),
			level,
		)
		cores = append(cores, fileCore)
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}
