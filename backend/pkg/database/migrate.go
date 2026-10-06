package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	embedmigrations "github.com/2m-topup/backend/pkg/migrate"
)

// RunMigrations applies all pending UP migrations embedded in the binary.
// It is idempotent — safe to call on every server start.
func RunMigrations(db *sql.DB) error {
	// Source: embedded SQL files via embed.FS
	src, err := iofs.New(embedmigrations.FS, "sql")
	if err != nil {
		return fmt.Errorf("create iofs source: %w", err)
	}

	// Driver: postgres
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Println("Migrations applied successfully")
	return nil
}
