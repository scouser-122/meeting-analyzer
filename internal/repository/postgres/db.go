package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scouser-122/meeting-analyzer/internal/config"
)

// DBPoolInterface specifies interface to interact with DB via connection pool
type DBPoolInterface interface {
	// Ping makes ping to DB and returns error if not succeeded
	Ping(ctx context.Context) error

	// Close closes DB connection
	Close()

	// Exec makes request which changes DB content
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)

	// QueryRow takes one row by request with specified parameters
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row

	// Query takes multiple rows by request with specified parameters
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)

	// Begin creates DB transaction
	Begin(ctx context.Context) (pgx.Tx, error)
}

// PostgresDatabase used to interact with Postgres DB
type PostgresDatabase struct {
	// Config DB connection config
	Config config.DBConnectionConfig
	pool   DBPoolInterface
}

// NewPostgresDB creates new PostgresDatabase object to interact with Postgres DB
func NewPostgresDB(serverConfig config.ServerConfig) PostgresDatabase {
	database := PostgresDatabase{
		Config: config.DBConnectionConfig{
			DSN:         *serverConfig.DBDataSourceName,
			RetryConfig: config.DefaultRetryConfig(),
		},
	}
	return database
}

// Open connects to Postgres DB and applies migrations
func (db *PostgresDatabase) Open() error {
	if db.Config.DSN == "" {
		return fmt.Errorf("connection string is empty")
	}

	var err error
	config, err := pgxpool.ParseConfig(db.Config.DSN)
	if err != nil {
		return fmt.Errorf("failed to parse connection string: %q, err: %w", db.Config.DSN, err)
	}

	db.pool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err = db.pool.Ping(context.Background()); err != nil {
		db.pool.Close()
		db.pool = nil
		return fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("successfully connected to DB")

	err = db.runMigrations()
	if err != nil {
		db.pool.Close()
		db.pool = nil
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func (db *PostgresDatabase) runMigrations() error {
	path, err := getMigrationsPath()
	if err != nil {
		return err
	}
	m, err := migrate.New(
		path,
		db.Config.DSN,
	)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	slog.Info("DB migrations completed successfully")
	return nil
}

func getMigrationsPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	execDir := filepath.Dir(execPath)

	migrationsPath := filepath.Join(execDir, "../../migrations")

	return "file://" + migrationsPath, nil
}

// Close closes DB connection
func (db *PostgresDatabase) Close() {
	if !isNil(db.pool) {
		db.pool.Close()
	}
}

// Ping make ping request to DB
func (db *PostgresDatabase) Ping(ctx context.Context) error {
	if isNil(db.pool) {
		return fmt.Errorf("database connection was not opened")
	}
	return DataBaseRequestRetry(
		ctx,
		db.Config.RetryConfig,
		func() error {
			return db.pool.Ping(ctx)
		},
	)
}

// Exec makes request which changes DB content
func (db *PostgresDatabase) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if isNil(db.pool) {
		return pgconn.CommandTag{}, fmt.Errorf("database connection was not opened")
	}
	var commandTag pgconn.CommandTag
	err := DataBaseRequestRetry(
		ctx,
		db.Config.RetryConfig,
		func() error {
			var err error
			commandTag, err = db.pool.Exec(ctx, query, args...)
			return err
		},
	)
	return commandTag, err
}

// Query takes multiple rows by request with specified parameters
func (db *PostgresDatabase) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if isNil(db.pool) {
		return nil, fmt.Errorf("database connection was not opened")
	}
	var rows pgx.Rows
	err := DataBaseRequestRetry(
		ctx,
		db.Config.RetryConfig,
		func() error {
			var err error
			rows, err = db.pool.Query(ctx, query, args...)
			return err
		},
	)
	return rows, err
}

// QueryRow takes one row by request with specified parameters
func (db *PostgresDatabase) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return db.pool.QueryRow(ctx, query, args...)
}

// Begin creates DB transaction
func (db *PostgresDatabase) Begin(ctx context.Context) (pgx.Tx, error) {
	if isNil(db.pool) {
		return nil, fmt.Errorf("database connection was not opened")
	}
	var tx pgx.Tx
	err := DataBaseRequestRetry(
		ctx,
		db.Config.RetryConfig,
		func() error {
			var err error
			tx, err = db.pool.Begin(ctx)
			return err
		},
	)
	return tx, err
}

func isNil(i any) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	}
	return false
}
