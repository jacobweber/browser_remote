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

func TestNativeReader(t *testing.T) {
	logger := logger.NewStdoutLogger()

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	// input/output formats are the same, so use another instance to simulate browser
	native := NewNativeMessaging[TestMessageFromBrowser, TestMessageToBrowser](&logger, inputReader, outputWriter)
	browser := NewNativeMessaging[TestMessageToBrowser, TestMessageFromBrowser](&logger, outputReader, inputWriter)

	nativeResponder := NewTestResponderFromBrowser()
	browserResponder := NewTestResponderToBrowser()

	nativeDone := make(chan bool)
	browserDone := make(chan bool)
	go func() {
		native.ReadMessagesFromBrowser(&nativeResponder)
		nativeDone <- true
	}()
	go func() {
		browser.ReadMessagesFromBrowser(&browserResponder)
		browserDone <- true
	}()

	native.SendToBrowser(TestMessageToBrowser{Question: "name"})
	msgToBrowser := <-browserResponder.messages
	if msgToBrowser.Question != "name" {
		t.Errorf("Invalid message sent to browser: %v", msgToBrowser.Question)
	}

	browser.SendToBrowser(TestMessageFromBrowser{Answer: "john"})
	msgToNative := <-nativeResponder.messages
	if msgToNative.Answer != "john" {
		t.Errorf("Invalid message received from browser: %v", msgToNative.Answer)
	}

	inputWriter.Close()
	outputWriter.Close()
	<-nativeDone
	<-browserDone
}
