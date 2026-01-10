package botrate

import (
	"time"

	"golang.org/x/time/rate"
)

// Config holds core configuration.
type Config struct {
	Limit         rate.Limit
	Window        time.Duration
	PageThreshold int
	QueueCap      int
}
