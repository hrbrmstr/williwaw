package main

import (
	"fmt"
	"strconv"
	"time"
)

func Format(n int64) string {
	in := strconv.FormatInt(n, 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}

func DegToCompass(deg float64) string {
	var directions = []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW", "N"}
	ix := int((deg + 22.5) / 45)
	if ix < 0 {
		ix = 0
	} else if ix >= len(directions) {
		ix = len(directions) - 1
	}
	return directions[ix]
}

func formatReading(reading ObsSt) map[string]any {
	if reading.FirmwareRevision == 0 {
		return map[string]any{
			"serial": "⌛️",
			"batt":   "⌛️",
			"temp":   "⌛️",
			"humid":  "⌛️",
			"lumos":  "⌛️",
			"press":  "⌛️",
			"insol":  "⌛️",
			"ultra":  "⌛️",
			"wind":   "⌛️",
			"wdir":   "⌛️",
			"when":   "⌛️",
		}
	} else {
		return map[string]any{
			"serial": reading.SerialNumber,
			"batt":   fmt.Sprintf("%.1f volts", reading.Obs[0][16]),
			"temp":   fmt.Sprintf("%.1f", reading.Obs[0][7]),
			"humid":  fmt.Sprintf("%.1f%%", lastReading.Obs[0][8]),
			"lumos":  Format(int64(reading.Obs[0][9])),
			"press":  strconv.FormatInt(int64(reading.Obs[0][6]), 10),
			"insol":  Format(int64(reading.Obs[0][11])),
			"ultra":  Format(int64(reading.Obs[0][10])),
			"wind":   fmt.Sprintf("%.1f", reading.Obs[0][2]),
			"wdir":   DegToCompass(reading.Obs[0][4]),
			"when":   time.Now().Format("2006-01-02 15:04:05"),
		}
	}
}

func formatHubStatus(hubStatus HubStatus) map[string]any {
	if hubStatus.Timestamp == 0 {
		return map[string]any{
			"hubsn":   "⌛️",
			"hubfirm": "⌛️",
			"uptime":  "⌛️",
		}
	} else {
		return map[string]any{
			"hubsn":   hubStatus.SerialNumber,
			"hubfirm": hubStatus.FirmwareRevision,
			"uptime":  strconv.FormatInt(hubStatus.Uptime, 10),
		}
	}
}

func isValidUnixTimestamp(ts time.Time) bool {
	return ts.Year() > 0 && ts.Year() <= 9999
}

func parseDateOrDateTime(input string) (time.Time, error) {
	var layouts = []string{
		"2006-01-02",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}

	var parsedTime time.Time
	var err error

	tsInt, err := strconv.Atoi(input)

	if err == nil {
		parsedTime = time.Unix(int64(tsInt), 0)
		if isValidUnixTimestamp(parsedTime) {
			return parsedTime, nil
		}
	} else {
		for _, layout := range layouts {
			parsedTime, err = time.Parse(layout, input)
			if err == nil {
				return parsedTime, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date/time: %v", input)
}
