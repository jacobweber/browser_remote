package shared

import (
	"encoding/binary"
	"time"
	"unsafe"
)

// Message from the browser to the native host.
type MessageFromBrowser struct {
	Id      string `json:"id"`
	Status  string `json:"status"`
	Results []any  `json:"results"`
}

// Message from the native host to the browser.
type MessageToBrowser struct {
	Id     string `json:"id"`
	Query  string `json:"query"`
	Tabs   string `json:"tabs"`
	Result any    `json:"result"`
}

// Request to the web server.
type MessageToWebServer struct {
	Query string `json:"query"`
	Tabs  string `json:"tabs"`
}

// Response from the web server.
type MessageFromWebServer struct {
	Status  string `json:"status"`
	Results []any  `json:"results"`
}

type Timer interface {
	StartTimer(time.Duration) <-chan time.Time
}

type RealTimer struct {
}

func (timer *RealTimer) StartTimer(dur time.Duration) <-chan time.Time {
	return time.After(dur)
}

func DetermineByteOrder() binary.ByteOrder {
	// determine native byte order so that we can read message size correctly
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		return binary.BigEndian
	} else {
		return binary.LittleEndian
	}
}
