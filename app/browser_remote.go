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
	// optional:
	"tabs": "front" (default) | "all"
}

Response format:
{
	"status": "ok" | "error message"
	// one per tab
	"results": [
		"https://www.google.com"
	]
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
	logger := logger.NewFile()
	defer logger.Cleanup()

	host := flag.String("host", "localhost", "web server hostname")
	port := flag.Int("port", 5555, "web server port")
	flag.Parse()
	argv := len(os.Args)
	if argv > 1 {
		logger.Trace.Printf("arg: %v", os.Args[1])
	}

	openPort, ok := network.FindFreePort(&logger, *host, *port, 10, true)
	if !ok {
		logger.Error.Printf("Unable to open port: %v:%v", *host, *port)
		return
	}

	messageReader := native_messaging.NewReader[shared.MessageFromBrowser](&logger, os.Stdin)
	messageWriter := native_messaging.NewWriter[shared.MessageToBrowser](&logger, os.Stdout)
	webServer := web_server.New(&logger)

	webServer.OnMessage(func(msg shared.MessageToBrowser) {
		messageWriter.SendMessage(msg)
	})
	messageReader.OnMessage(func(msg shared.MessageFromBrowser) {
		webServer.HandleMessage(msg)
	})

	webServer.Start(*host, openPort)
	done := make(chan bool)
	go func() {
		messageReader.Start()
		done <- true
	}()
	go messageWriter.Start()
	<-done
	messageWriter.Done()
}
