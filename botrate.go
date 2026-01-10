package botrate

import "context"

// ErrLimit is returned when the request is rate limited.
var ErrLimit = context.DeadlineExceeded
