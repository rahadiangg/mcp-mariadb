package database

import (
	"context"
	"testing"
	"time"
)

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{
			name: "basic DSN",
			cfg: &Config{
				User:     "testuser",
				Password: "testpass",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
			},
			want: "testuser:testpass@tcp(localhost:3306)/testdb?multiStatements=false",
		},
		{
			name: "DSN with charset",
			cfg: &Config{
				User:     "testuser",
				Password: "testpass",
				Host:     "localhost",
				Port:     3306,
				Database: "testdb",
				Charset:  "utf8mb4",
			},
			want: "testuser:testpass@tcp(localhost:3306)/testdb?multiStatements=false&charset=utf8mb4",
		},
		{
			name: "DSN without database",
			cfg: &Config{
				User:     "testuser",
				Password: "testpass",
				Host:     "localhost",
				Port:     3306,
			},
			want: "testuser:testpass@tcp(localhost:3306)/?multiStatements=false",
		},
		{
			name: "DSN with SSL",
			cfg: &Config{
				User:              "testuser",
				Password:          "testpass",
				Host:              "localhost",
				Port:              3306,
				Database:          "testdb",
				SSL:               true,
				SSLVerifyCert:     false,
				SSLVerifyIdentity: false,
			},
			want: "testuser:testpass@tcp(localhost:3306)/testdb?multiStatements=false&tls=false",
		},
		{
			name: "DSN with SSL CA (with verify)",
			cfg: &Config{
				User:          "testuser",
				Password:      "testpass",
				Host:          "localhost",
				Port:          3306,
				Database:      "testdb",
				SSL:           true,
				SSLCA:         "/path/to/ca.pem",
				SSLVerifyCert: true,
			},
			want: "testuser:testpass@tcp(localhost:3306)/testdb?multiStatements=false&tls=true&ssl-ca=/path/to/ca.pem",
		},
		{
			name: "DSN with timeouts",
			cfg: &Config{
				User:          "testuser",
				Password:      "testpass",
				Host:          "localhost",
				Port:          3306,
				Database:      "testdb",
				ConnTimeout:   10 * time.Second,
				ReadTimeout:   30 * time.Second,
				WriteTimeout:  30 * time.Second,
			},
			want: "testuser:testpass@tcp(localhost:3306)/testdb?multiStatements=false&timeout=10s&readTimeout=30s&writeTimeout=30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDSN(tt.cfg)
			if got != tt.want {
				t.Errorf("buildDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewPoolErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing user",
			cfg: &Config{
				Host:     "localhost",
				Port:     3306,
				Password: "testpass",
			},
			wantErr: true,
			errMsg:  "database user is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := NewPool(ctx, tt.cfg, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("NewPool() error = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestSafePoolDB(t *testing.T) {
	pool := &SafePool{}
	if pool.DB() != nil {
		t.Error("Expected DB() to return nil for uninitialized pool")
	}
}

func TestSafePoolCloseNil(t *testing.T) {
	pool := &SafePool{}
	err := pool.Close()
	if err != nil {
		t.Errorf("Expected Close() on nil pool to return nil, got %v", err)
	}
}
