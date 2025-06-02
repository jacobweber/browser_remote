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

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	// input/output formats are the same, so use another instance to simulate browser
	nativeReader := NewNativeMessagingReader[TestMessageFromBrowser](&logger, inputReader)
	nativeWriter := NewNativeMessagingWriter[TestMessageToBrowser](&logger, outputWriter)
	browserReader := NewNativeMessagingReader[TestMessageToBrowser](&logger, outputReader)
	browserWriter := NewNativeMessagingWriter[TestMessageFromBrowser](&logger, inputWriter)

	nativeResponder := NewTestResponderFromBrowser()
	browserResponder := NewTestResponderToBrowser()

	nativeReaderDone := make(chan bool)
	browserReaderDone := make(chan bool)
	go func() {
		nativeReader.Start(&nativeResponder)
		nativeReaderDone <- true
	}()
	go func() {
		browserReader.Start(&browserResponder)
		browserReaderDone <- true
	}()
	go nativeWriter.Start()
	go browserWriter.Start()

	nativeWriter.SendMessage(TestMessageToBrowser{Question: "name"})
	msgToBrowser := <-browserResponder.messages
	if msgToBrowser.Question != "name" {
		t.Errorf("Invalid message sent to browser: %v", msgToBrowser.Question)
	}

	browserWriter.SendMessage(TestMessageFromBrowser{Answer: "john"})
	msgToNative := <-nativeResponder.messages
	if msgToNative.Answer != "john" {
		t.Errorf("Invalid message received from browser: %v", msgToNative.Answer)
	}

	inputWriter.Close()
	outputWriter.Close()
	<-nativeReaderDone
	<-browserReaderDone
	nativeWriter.Done()
	browserWriter.Done()
}
