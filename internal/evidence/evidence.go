package evidence

import "time"

type Evidence struct {
	Sent     string        `json:"sent"`
	Received string        `json:"received"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}
