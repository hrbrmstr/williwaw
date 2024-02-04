package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"log/slog"
	"time"
)

type PlotReading struct {
	Timestamp string  `json:"timestamp"`
	Temp      float64 `json:"temp"`
	Humid     float64 `json:"humid"`
	Lumos     int64   `json:"lumos"`
	Press     float64 `json:"press"`
}

// DATETIME('now', '-36 hours')

const (
	sinceQueryString = `
SELECT
  datetime(json_extract(reading, '$.obs[0][0]'), 'unixepoch') as ts,
  json_extract(reading, '$.obs[0][7]') as temp,
  json_extract(reading, '$.obs[0][8]') as humid,
  json_extract(reading, '$.obs[0][9]') as lumos,
  json_extract(reading, '$.obs[0][6]') as press
FROM readings 
WHERE 
  timestamp >= ?
ORDER BY
  datetime(json_extract(reading, '$.obs[0][0]'), 'unixepoch')
`
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

func sinceQuery(ts time.Time) (string, error) {

	rows, err := db.Query(sinceQueryString, ts)

	if err != nil {
		return "[]", err
	}
	defer rows.Close()

	var readings []PlotReading

	// Iterate through the result set
	for rows.Next() {
		var r PlotReading
		err = rows.Scan(&r.Timestamp, &r.Temp, &r.Humid, &r.Lumos, &r.Press)
		if err != nil {
			return "[]", err
		}
		readings = append(readings, r)
	}

	// Check for errors from iterating over rows
	if err = rows.Err(); err != nil {
		return "[]", err
	}

	// Convert the result set to JSON
	jsonData, err := json.Marshal(readings)
	if err != nil {
		return "[]", err
	}

	return string(jsonData), nil
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
