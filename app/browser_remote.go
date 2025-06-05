package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jacobweber/browser_remote/internal/logger"
	"github.com/jacobweber/browser_remote/internal/native_messaging"
	"github.com/jacobweber/browser_remote/internal/network"
	"github.com/jacobweber/browser_remote/internal/shared"
	"github.com/jacobweber/browser_remote/internal/web_server"
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

	openPort, ok := network.FindFreePort(logger, *host, *port, 10, true)
	if !ok {
		logger.Error.Printf("Unable to open port: %v:%v", *host, *port)
		return
	}

	messageReader := native_messaging.NewReader[shared.MessageFromBrowser](logger, os.Stdin, "from browser")
	messageWriter := native_messaging.NewWriter[shared.MessageToBrowser](logger, os.Stdout, "to browser")
	webServer := web_server.New(logger)

	webServer.OnMessageReadyForBrowser(func(msg shared.MessageToBrowser) {
		messageWriter.SendMessage(msg)
	})
	messageReader.OnMessageRead(func(msg shared.MessageFromBrowser) {
		webServer.HandleMessageFromBrowser(msg)
	})

	webServer.Start(*host, openPort)
	done := make(chan bool)
	go func() {
		messageReader.Start()
		done <- true
	}()
	go messageWriter.Start()

	messageWriter.SendMessage(shared.MessageToBrowser{
		Id: "status",
		Result: map[string]any{
			"address": fmt.Sprintf("http://%v:%v", *host, openPort),
		},
	})

	<-done
	messageWriter.Done()
}
