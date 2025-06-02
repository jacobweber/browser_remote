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

type TestResponderFromBrowser struct {
	messages chan TestMessageFromBrowser
}

func NewTestResponderFromBrowser() TestResponderFromBrowser {
	return TestResponderFromBrowser{
		messages: make(chan TestMessageFromBrowser),
	}
}

func (resp *TestResponderFromBrowser) HandleMessage(incomingMsg TestMessageFromBrowser) {
	resp.messages <- incomingMsg
}

type TestResponderToBrowser struct {
	messages chan TestMessageToBrowser
}

func NewTestResponderToBrowser() TestResponderToBrowser {
	return TestResponderToBrowser{
		messages: make(chan TestMessageToBrowser),
	}
}

func (resp *TestResponderToBrowser) HandleMessage(incomingMsg TestMessageToBrowser) {
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

	nativeResponder := NewTestResponderFromBrowser()
	browserResponder := NewTestResponderToBrowser()

	readerFromBrowserDone := make(chan bool)
	readerFromNativeDone := make(chan bool)
	go func() {
		messageReaderFromBrowser.Start(&nativeResponder)
		readerFromBrowserDone <- true
	}()
	go func() {
		messageReaderFromNative.Start(&browserResponder)
		readerFromNativeDone <- true
	}()
	go messageWriterToBrowser.Start()
	go messageWriterToNative.Start()

	messageWriterToBrowser.SendMessage(TestMessageToBrowser{Question: "name"})
	msgToBrowser := <-browserResponder.messages
	if msgToBrowser.Question != "name" {
		t.Errorf("Invalid message sent to browser: %v", msgToBrowser.Question)
	}

	messageWriterToNative.SendMessage(TestMessageFromBrowser{Answer: "john"})
	msgToNative := <-nativeResponder.messages
	if msgToNative.Answer != "john" {
		t.Errorf("Invalid message received from browser: %v", msgToNative.Answer)
	}

	writerToNative.Close()
	writerToBrowser.Close()
	<-readerFromBrowserDone
	<-readerFromNativeDone
	messageWriterToBrowser.Done()
	messageWriterToNative.Done()
}
