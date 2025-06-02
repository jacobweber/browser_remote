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
	"example/remote/internal/shared"
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

	messageReader := native_messaging.NewNativeMessagingReader[shared.MessageFromBrowser](&logger, os.Stdin)
	messageWriter := native_messaging.NewNativeMessagingWriter[shared.MessageToBrowser](&logger, os.Stdout)
	webServer := web_server.NewWebServer(&logger, &messageWriter)

	webServer.Start(*host, openPort)
	go messageReader.Start(&webServer)
	go messageWriter.Start()
}
