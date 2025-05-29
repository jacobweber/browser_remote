package main

import (
	"fmt"
	"net"
	"time"
)

func FindFreePort(host string, port int, maxTries int, increment bool) (int, bool) {
	timeout := time.Second / 2
	for i := 0; i < maxTries; i++ {
		if !isPortInUse(host, port, timeout) {
			return port, true
		}
		Error.Printf("Unable to open port: %v:%v; retrying", host, port)
		time.Sleep(timeout)
		if increment {
			port++
		}
	}
	return port, false
}

func isPortInUse(host string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%v:%v", host, port), timeout)
	if err != nil || conn == nil {
		return false
	}
	conn.Close()
	return true
}
