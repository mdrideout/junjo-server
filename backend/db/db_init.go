// File: db_init.go

package db

import (
	"context"
	"database/sql"
	"embed" // Import the 'embed' package to include migration files in the binary
	"log/slog"
	"os"

	"github.com/pressly/goose/v3" // Import the Goose library
	_ "modernc.org/sqlite"
)

// The //go:embed directive tells the Go compiler to embed the contents of the
// 'migrations' directory into the 'embedMigrations' variable.
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

// Global database connection variable.
var DB *sql.DB

// Connect initializes the database connection and runs all pending migrations.
func Connect() {
	ctx := context.Background()
	dbPath := "/dbdata/sqlite/app_data.db"

	// Open the database connection using the sqlite driver.
	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		slog.Error("failed to open database", slog.String("path", dbPath), slog.Any("error", err))
		os.Exit(1)
	}

	if err = DB.Ping(); err != nil {
		slog.Error("failed to connect to database", slog.String("path", dbPath), slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("successfully connected to sqlite database", slog.String("path", dbPath))

	// --- Goose Migration Logic ---
	// Tell Goose to use the embedded files as the source for migrations.
	goose.SetBaseFS(embedMigrations)

	// Set the database dialect for Goose.
	if err := goose.SetDialect("sqlite"); err != nil {
		slog.Error("failed to set goose dialect", slog.Any("error", err))
		os.Exit(1)
	}

	// Run all 'up' migrations. Goose keeps track of which migrations have
	// already been applied and only runs the new ones.
	if err := goose.Up(DB, "migrations"); err != nil {
		slog.Error("failed to run goose migrations", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("database migrations applied successfully")

	// --- Safety & Performance PRAGMAs ---
	// Use DELETE mode (traditional rollback journal) for maximum safety and simplicity.
	// This is appropriate for low-throughput, critical data like users and API keys.
	_, err = DB.ExecContext(ctx, "PRAGMA journal_mode=DELETE")
	if err != nil {
		slog.Error("failed to set journal mode", slog.Any("error", err))
		os.Exit(1)
	}

	// Use FULL synchronous mode to ensure every transaction is fully synced to disk
	// before the commit returns. This prevents data loss on power failure or crash.
	_, err = DB.ExecContext(ctx, "PRAGMA synchronous=FULL")
	if err != nil {
		slog.Error("failed to set synchronous mode", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("sqlite configured for maximum data safety", slog.String("journal_mode", "DELETE"), slog.String("synchronous", "FULL"))
}

// Close safely closes the database connection.
func Close() {
	if DB != nil {
		DB.Close()
	}
}
