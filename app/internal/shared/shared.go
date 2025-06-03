package shared

import (
	"encoding/binary"
	"time"
	"unsafe"
)

// Request from the browser to the native host.
type MessageFromBrowser struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Result any    `json:"result"`
}

// Response from the native host to the browser.
type MessageToBrowser struct {
	Id    string `json:"id"`
	Query string `json:"query"`
}

// Request to the web server.
type MessageToWebServer struct {
	Query string `json:"query"`
}

// Response from the web server.
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
