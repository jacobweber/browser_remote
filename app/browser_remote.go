/*
Opens web server on port 5555, or next free port, or port specified with --port=<port>.
Send requests to the web server, and they'll be evaluated in the front browser window, and returned.

Request format:
POST /
{
	// an expression to evaluate and return:
	"query": "location.href"
	// or a function call:
	"query": "window.open(\"https://www.apple.com\")"
}

Response format:
{
	"status": "ok" | "error message"
	"result": "https://www.google.com"
}
*/

package main

import (
	"example/remote/internal/logger"
	"example/remote/internal/native_messaging"
	"example/remote/internal/network"
	"example/remote/internal/web_server"
	"flag"
	"os"
)

func main() {
	logger := logger.NewFileLogger()
	defer logger.Cleanup()

	host := flag.String("host", "localhost", "web server hostname")
	port := flag.Int("port", 5555, "web server port")
	flag.Parse()
	argv := len(os.Args)
	if argv > 1 {
		logger.Trace.Printf("arg: %v", os.Args[1])
	}

	openPort, ok := network.FindFreePort(*host, *port, 10, true)
	if !ok {
		logger.Error.Printf("Unable to open port: %v:%v", *host, *port)
		return
	}

	nm := native_messaging.NewNativeMessaging[web_server.IncomingBrowserMessage, web_server.OutgoingBrowserMessage](&logger, os.Stdin, os.Stdout)
	ws := web_server.NewWebServer(&logger, *host, openPort, &nm)

	ws.Start()
	nm.Start(&ws)
}
