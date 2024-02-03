package main

import (
	"database/sql"
	"log"
	"log/slog"
	"time"
)

func initDB(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatalf("failed to init db: %s", err)
	}
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS readings (
      timestamp DATETIME PRIMARY KEY,
			record_type TEXT NOT NULL,
      reading BLOB NOT NULL
    )
`)
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("DB initialized")
	return db
}

func logReading(recordType string, reading []byte) {
	timestamp := time.Now()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	stmt, err := tx.Prepare("insert into readings(timestamp, record_type, reading) values(?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(timestamp, recordType, reading)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}
