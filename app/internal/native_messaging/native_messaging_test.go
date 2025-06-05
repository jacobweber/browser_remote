package native_messaging

import (
	"io"
	"testing"

	"github.com/jacobweber/browser_remote/internal/logger"
)

type TestMessageFromBrowser struct {
	Answer string `json:"answer"`
}

type TestMessageToBrowser struct {
	Question string `json:"question"`
}

type TestMessageFromBrowserHandler struct {
	messages chan TestMessageFromBrowser
}

func NewTestMessageFromBrowserHandler() TestMessageFromBrowserHandler {
	return TestMessageFromBrowserHandler{
		messages: make(chan TestMessageFromBrowser),
	}
}

func (resp *TestMessageFromBrowserHandler) HandleMessage(incomingMsg TestMessageFromBrowser) {
	resp.messages <- incomingMsg
}

type TestMessageFromNativeHandler struct {
	messages chan TestMessageToBrowser
}

func NewTestMessageFromNativeHandler() TestMessageFromNativeHandler {
	return TestMessageFromNativeHandler{
		messages: make(chan TestMessageToBrowser),
	}
}

func (resp *TestMessageFromNativeHandler) HandleMessage(incomingMsg TestMessageToBrowser) {
	resp.messages <- incomingMsg
}

func TestNativeMessaging(t *testing.T) {
	logger := logger.NewStdout()

	readerFromBrowser, writerToNative := io.Pipe()
	readerFromNative, writerToBrowser := io.Pipe()

	messageReaderFromBrowser := NewReader[TestMessageFromBrowser](logger, readerFromBrowser, "from browser")
	messageWriterToBrowser := NewWriter[TestMessageToBrowser](logger, writerToBrowser, "to browser")
	// input/output formats are the same, so use another instance to simulate browser
	messageReaderFromNative := NewReader[TestMessageToBrowser](logger, readerFromNative, "from native")
	messageWriterToNative := NewWriter[TestMessageFromBrowser](logger, writerToNative, "to native")

	messageFromBrowserHandler := NewTestMessageFromBrowserHandler()
	messageFromNativeHandler := NewTestMessageFromNativeHandler()
	messageReaderFromBrowser.OnMessageRead(func(msg TestMessageFromBrowser) {
		messageFromBrowserHandler.HandleMessage(msg)
	})
	messageReaderFromNative.OnMessageRead(func(msg TestMessageToBrowser) {
		messageFromNativeHandler.HandleMessage(msg)
	})

	readerFromBrowserDone := make(chan bool)
	readerFromNativeDone := make(chan bool)
	go func() {
		messageReaderFromBrowser.Start()
		readerFromBrowserDone <- true
	}()
	go func() {
		messageReaderFromNative.Start()
		readerFromNativeDone <- true
	}()
	go messageWriterToBrowser.Start()
	go messageWriterToNative.Start()

	messageWriterToBrowser.SendMessage(TestMessageToBrowser{Question: "name"})
	messageFromNative := <-messageFromNativeHandler.messages
	if messageFromNative.Question != "name" {
		t.Errorf("Invalid message received from native: %v", messageFromNative.Question)
	}

	messageWriterToNative.SendMessage(TestMessageFromBrowser{Answer: "john"})
	messageFromBrowser := <-messageFromBrowserHandler.messages
	if messageFromBrowser.Answer != "john" {
		t.Errorf("Invalid message received from browser: %v", messageFromBrowser.Answer)
	}

	writerToNative.Close()
	writerToBrowser.Close()
	<-readerFromBrowserDone
	<-readerFromNativeDone
	messageWriterToBrowser.Done()
	messageWriterToNative.Done()
}
