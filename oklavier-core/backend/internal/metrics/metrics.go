package metrics

import "sync/atomic"

// HTTP request counters by status class
var (
	HTTPRequests2xx      atomic.Int64
	HTTPRequests4xx      atomic.Int64
	HTTPRequests5xx      atomic.Int64
	SessionsCreatedTotal atomic.Int64
)
