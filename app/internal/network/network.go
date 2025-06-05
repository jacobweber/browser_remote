package network

import (
	"fmt"
	"net"
	"time"

	"github.com/jacobweber/browser_remote/internal/logger"
)

func FindFreePort(logger *logger.Logger, host string, port int, maxTries int, increment bool) (int, bool) {
	timeout := time.Second / 2
	for i := 0; i < maxTries; i++ {
		if !isPortInUse(host, port, timeout) {
			return port, true
		}
		logger.Error.Printf("Unable to open port: %v:%v; trying another port", host, port)
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
