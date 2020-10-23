package internal

import "time"

var timeFormats = []string{
	// By default, store timestamps with whatever timezone they come with.
	// When parsed, they will be returned with the same timezone.
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

func parseTimestamp(ts string) (t time.Time, err error) {
	for _, format := range timeFormats {
		if t, err = time.Parse(format, ts); err == nil {
			return t, nil
		}
	}
	return t, err
}
