package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// SafePool wraps sqlx.DB with safe connection settings (MULTI_STATEMENTS disabled)
type SafePool struct {
	db     *sqlx.DB
	logger *zap.Logger
	config *Config
}

// Config holds database connection configuration
type Config struct {
	Host              string
	Port              int
	User              string
	Password          string
	Database          string
	Charset           string
	SSL               bool
	SSLCA             string
	SSLCert           string
	SSLKey            string
	SSLVerifyCert     bool
	SSLVerifyIdentity bool
	MaxOpenConns      int
	ConnMaxLifetime   time.Duration
}

// NewPool creates a new safe connection pool
// IMPORTANT: Always sets multiStatements=false in the DSN to prevent SQL injection
func NewPool(ctx context.Context, cfg *Config, logger *zap.Logger) (*SafePool, error) {
	if cfg.User == "" {
		return nil, fmt.Errorf("database user is required")
	}

	// Build DSN with multiStatements=false to disable multiple statements
	dsn := buildDSN(cfg)

	logger.Info("Creating database connection pool",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("user", cfg.User),
		zap.String("database", cfg.Database),
		zap.Bool("ssl", cfg.SSL),
	)

	db, err := sqlx.ConnectContext(ctx, "safemysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure pool settings
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
		db.SetMaxIdleConns(cfg.MaxOpenConns / 2)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection pool created successfully")

	return &SafePool{
		db:     db,
		logger: logger,
		config: cfg,
	}, nil
}

// buildDSN constructs the MySQL Data Source Name with safe defaults
// Always includes multiStatements=false
func buildDSN(cfg *Config) string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
	)

	if cfg.Database != "" {
		dsn += "/" + cfg.Database
	} else {
		dsn += "/"
	}

	// Build parameters
	params := []string{"multiStatements=false"}

	if cfg.Charset != "" {
		params = append(params, "charset="+cfg.Charset)
	}

	// SSL configuration
	if cfg.SSL {
		sslMode := "true"
		if !cfg.SSLVerifyCert {
			sslMode = "false"
		}
		params = append(params, "tls="+sslMode)

		if cfg.SSLCA != "" {
			params = append(params, "ssl-ca="+cfg.SSLCA)
		}
		if cfg.SSLCert != "" {
			params = append(params, "ssl-cert="+cfg.SSLCert)
		}
		if cfg.SSLKey != "" {
			params = append(params, "ssl-key="+cfg.SSLKey)
		}
	}

	// Append parameters
	if len(params) > 0 {
		dsn += "?"
		for i, p := range params {
			if i > 0 {
				dsn += "&"
			}
			dsn += p
		}
	}

	return dsn
}

// Close closes the database connection pool
func (p *SafePool) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// DB returns the underlying sqlx.DB
func (p *SafePool) DB() *sqlx.DB {
	return p.db
}

// Conn returns a single connection from the pool
func (p *SafePool) Conn(ctx context.Context) (*sqlx.Conn, error) {
	return p.db.Connx(ctx)
}

// Acquire acquires a connection from the pool for use with transaction
type Conn struct {
	*sqlx.Conn
	pool *SafePool
}

// WithContext runs a function with a connection from the pool
func (p *SafePool) WithContext(ctx context.Context, fn func(*sqlx.Conn) error) error {
	conn, err := p.db.Connx(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return fn(conn)
}

// BeginTxCtx begins a transaction
func (p *SafePool) BeginTxCtx(ctx context.Context) (*sqlx.Tx, error) {
	return p.db.BeginTxx(ctx, nil)
}

// SelectContext executes a SELECT query and returns results
func (p *SafePool) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return p.db.SelectContext(ctx, dest, query, args...)
}

// GetContext executes a query that returns a single row
func (p *SafePool) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return p.db.GetContext(ctx, dest, query, args...)
}

// QueryxContext executes a query and returns rows
func (p *SafePool) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return p.db.QueryxContext(ctx, query, args...)
}

// ExecContext executes a query without returning rows
func (p *SafePool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}
