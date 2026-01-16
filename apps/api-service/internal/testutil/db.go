// Package testutil provides testing utilities for the api-service.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// TestDB wraps a database connection for testing purposes.
type TestDB struct {
	DB *sql.DB
}

// SetupTestDB creates a new database connection for testing.
// It reads connection parameters from environment variables:
//   - TEST_DB_HOST (default: localhost)
//   - TEST_DB_PORT (default: 5432)
//   - TEST_DB_USER (default: postgres)
//   - TEST_DB_PASSWORD (default: Vb96vpmAEZVdSpruALAgDmZrHFRDXATg)
//   - TEST_DB_NAME (default: beef_briefing)
//   - TEST_DB_SSLMODE (default: disable)
//
// Returns a TestDB wrapper that should be cleaned up with TeardownTestDB.
func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()

	host := getEnvOrDefault("TEST_DB_HOST", "localhost")
	port := getEnvOrDefault("TEST_DB_PORT", "5432")
	user := getEnvOrDefault("TEST_DB_USER", "postgres")
	password := getEnvOrDefault("TEST_DB_PASSWORD", "Vb96vpmAEZVdSpruALAgDmZrHFRDXATg")
	dbname := getEnvOrDefault("TEST_DB_NAME", "beef_briefing")
	sslmode := getEnvOrDefault("TEST_DB_SSLMODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Configure connection pool for testing
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	// Verify connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}

	return &TestDB{DB: db}
}

// TeardownTestDB closes the test database connection and performs cleanup.
func TeardownTestDB(t *testing.T, tdb *TestDB) {
	t.Helper()

	if tdb == nil || tdb.DB == nil {
		return
	}

	if err := tdb.DB.Close(); err != nil {
		t.Errorf("failed to close test database: %v", err)
	}
}

// WithTestTransaction executes a test function within a database transaction
// that is always rolled back after the test completes.
// This provides test isolation without modifying the database state.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    tdb := testutil.SetupTestDB(t)
//	    defer testutil.TeardownTestDB(t, tdb)
//
//	    testutil.WithTestTransaction(t, tdb.DB, func(tx *sql.Tx) {
//	        // Test code using tx
//	        // Changes will be rolled back after this function returns
//	    })
//	}
func WithTestTransaction(t *testing.T, db *sql.DB, fn func(tx *sql.Tx)) {
	t.Helper()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin test transaction: %v", err)
	}

	// Always rollback - we never want test transactions to commit
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			t.Errorf("failed to rollback test transaction: %v", err)
		}
	}()

	// Handle panics to ensure rollback
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("test panicked: %v", r)
		}
	}()

	fn(tx)
}

// WithTestTransactionContext is like WithTestTransaction but accepts a context.
// The context can be used to control timeouts and cancellation.
func WithTestTransactionContext(t *testing.T, ctx context.Context, db *sql.DB, fn func(ctx context.Context, tx *sql.Tx)) {
	t.Helper()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin test transaction: %v", err)
	}

	// Always rollback - we never want test transactions to commit
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			t.Errorf("failed to rollback test transaction: %v", err)
		}
	}()

	// Handle panics to ensure rollback
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("test panicked: %v", r)
		}
	}()

	fn(ctx, tx)
}

// getEnvOrDefault returns the value of an environment variable or a default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// CleanupTables truncates the specified tables within a transaction.
// Useful for resetting state between tests when not using WithTestTransaction.
func CleanupTables(t *testing.T, tx *sql.Tx, tables ...string) {
	t.Helper()

	for _, table := range tables {
		// Use TRUNCATE with CASCADE to handle foreign key constraints
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := tx.Exec(query); err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}

// DBTX is an interface that is satisfied by both *sql.DB and *sql.Tx.
// Use this type when writing repository methods that need to work with
// either a database connection or a transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Ensure *sql.DB and *sql.Tx satisfy DBTX at compile time
var (
	_ DBTX = (*sql.DB)(nil)
	_ DBTX = (*sql.Tx)(nil)
)
