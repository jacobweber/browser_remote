package native_messaging

import (
	"example/remote/internal/logger"
	"io"
	"testing"
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
	logger := logger.NewStdoutLogger()

	readerFromBrowser, writerToNative := io.Pipe()
	readerFromNative, writerToBrowser := io.Pipe()

	messageReaderFromBrowser := NewNativeMessagingReader[TestMessageFromBrowser](&logger, readerFromBrowser)
	messageWriterToBrowser := NewNativeMessagingWriter[TestMessageToBrowser](&logger, writerToBrowser)
	// input/output formats are the same, so use another instance to simulate browser
	messageReaderFromNative := NewNativeMessagingReader[TestMessageToBrowser](&logger, readerFromNative)
	messageWriterToNative := NewNativeMessagingWriter[TestMessageFromBrowser](&logger, writerToNative)

	messageFromBrowserHandler := NewTestMessageFromBrowserHandler()
	messageFromNativeHandler := NewTestMessageFromNativeHandler()

	readerFromBrowserDone := make(chan bool)
	readerFromNativeDone := make(chan bool)
	go func() {
		messageReaderFromBrowser.Start(&messageFromBrowserHandler)
		readerFromBrowserDone <- true
	}()
	go func() {
		messageReaderFromNative.Start(&messageFromNativeHandler)
		readerFromNativeDone <- true
	}()
	go messageWriterToBrowser.Start()
	go messageWriterToNative.Start()

	messageWriterToBrowser.SendMessage(TestMessageToBrowser{Question: "name"})
	messageFromNative := <-messageFromNativeHandler.messages
	if messageFromNative.Question != "name" {
		t.Errorf("Invalid message sent to browser: %v", messageFromNative.Question)
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
