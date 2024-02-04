package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"log/slog"
	"time"
)

// 0	1588948614  Time Epoch	Seconds
// 1	0.18        Wind Lull (minimum 3 second sample)	m/s
// 2	0.22        Wind Avg (average over report interval)	m/s
// 3	0.27        Wind Gust (maximum 3 second sample)	m/s
// 4	144         Wind Direction	Degrees
// 5	6           Wind Sample Interval	seconds
// 6	1017.57     Station Pressure	MB
// 7	22.37       Air Temperature	C
// 8	50.26       Relative Humidity	%
// 9	328         Illuminance	Lux
// 10	0.03        UV	Index
// 11	3           Solar Radiation	W/m^2
// 12	0.000000    Rain amount over previous minute	mm
// 13	0           Precipitation Type	0 = none, 1 = rain, 2 = hail, 3 = rain + hail (experimental)
// 14	0           Lightning Strike Avg Distance	km
// 15	0           Lightning Strike Count
// 16	2.410       Battery	Volts
// 17	1           Report Interval	Minutes

// type PlotReading struct {
// 	Timestamp                  string  `json:"timestamp"`
// 	WindLull                   float64 `json:"wind_lull"`
// 	WindAvg                    float64 `json:"wind_avg"`
// 	WindGust                   float64 `json:"wind_gust"`
// 	WindDir                    int64   `json:"wind_direction"`
// 	WindSampleInterval         int64   `json:"wind_interval"`
// 	Press                      float64 `json:"press"`
// 	Temp                       float64 `json:"temp"`
// 	Humid                      float64 `json:"humid"`
// 	Lumos                      int64   `json:"lumos"`
// 	UV                         float64 `json:"uv"`
// 	SolarRad                   int64   `json:"solar_rad"`
// 	Rain1m                     float64 `json:"rain1m"`
// 	PrecipType                 int64   `json:"precip_type"`
// 	LightningStrikeAvgDistance int64   `json:"lightning_strike_avg_distance"`
// 	LightningStrikeCount       int64   `json:"lightning_strike_count"`
// 	Volts                      float64 `json:"volts"`
// 	ReportInterval             int64   `json:"report_interval"`
// }

type PlotReading struct {
	Timestamp string  `json:"timestamp"`
	Temp      float64 `json:"temp"`
	Humid     float64 `json:"humid"`
	Lumos     int64   `json:"lumos"`
	Press     float64 `json:"press"`
}

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

// handle API query for readings since a given timestamp
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

// store readings in the db
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
