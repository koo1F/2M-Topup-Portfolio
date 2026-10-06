package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq" // register postgres driver
)

// NewPostgres creates a PostgreSQL connection pool and verifies connectivity.
// It retries up to 5 times to handle the case where the DB container is still starting.
func NewPostgres(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Retry ping up to 10 times — useful when postgres container is still initialising
	for i := range 10 {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("postgres not ready (attempt %d/10): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres unreachable after retries: %w", err)
	}

	log.Println("PostgreSQL connected")
	return db, nil
}

// OpenPostgres initializes a Postgres connection pool without doing synchronous pings,
// which is useful for starting the server immediately and retrying in the background.
func OpenPostgres(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
