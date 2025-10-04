package utils

import (
	"time"
)

func GetCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return d.String()
	}
	if d < time.Minute {
		return d.Truncate(time.Millisecond).String()
	}
	if d < time.Hour {
		return d.Truncate(time.Second).String()
	}
	return d.Truncate(time.Minute).String()
}

func ParseTimestamp(timestamp string) (time.Time, error) {
	return time.Parse(time.RFC3339, timestamp)
}