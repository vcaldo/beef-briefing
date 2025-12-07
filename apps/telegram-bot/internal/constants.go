package internal

import "time"

// HTTP client timeouts
const (
	DefaultHTTPTimeout  = 30 * time.Second
	FileDownloadTimeout = 2 * time.Minute
)

// Retry configuration
const (
	MaxRetryAttempts       = 3
	MaxConcurrentDownloads = 5
)

// RetryDelays defines exponential backoff delays for retry attempts
var RetryDelays = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

// File size limits
const (
	MaxFileSize = 100 * 1024 * 1024 // 100MB
)
