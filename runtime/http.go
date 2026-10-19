package runtime

import (
	"net/http"
	"time"
)

// HTTP holds the http.Clients used for outbound calls
type HTTP struct {
	Courier *http.Client
}

func newHTTP() *HTTP {
	// a visitor's browser is waiting on each call to courier, so give up well before they would
	return &HTTP{
		Courier: &http.Client{Timeout: 15 * time.Second},
	}
}
