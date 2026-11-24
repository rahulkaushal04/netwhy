// Package evidence holds the proof a probe attaches to its result.
package evidence

import (
	"encoding/json"
	"time"
)

type Evidence struct {
	Sent     string
	Received string
	Duration time.Duration
	Error    string
}

// MarshalJSON writes the duration in milliseconds, since a time.Duration
// would appear as an unreadable count of nanoseconds.
func (e Evidence) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sent       string  `json:"sent"`
		Received   string  `json:"received"`
		DurationMS float64 `json:"duration_ms"`
		Error      string  `json:"error,omitempty"`
	}{e.Sent, e.Received, float64(e.Duration) / float64(time.Millisecond), e.Error})
}
