package shared

import (
	"time"
)

// MessageFromBrowser represents a message from the browser to the native host.
type MessageFromBrowser struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Result any    `json:"result"`
}

// MessageToBrowser respresents a response from the native host to the browser.
type MessageToBrowser struct {
	Id    string `json:"id"`
	Query string `json:"query"`
}

// MessageToWebServer represents a message to the web server.
type MessageToWebServer struct {
	Query string `json:"query"`
}

// MessageFromWebServer respresents a response from the web server.
type MessageFromWebServer struct {
	Status string `json:"status"`
	Result any    `json:"result"`
}

type Timer interface {
	StartTimer(time.Duration) <-chan time.Time
}

type RealTimer struct {
}

func (timer *RealTimer) StartTimer(dur time.Duration) <-chan time.Time {
	return time.After(dur)
}
