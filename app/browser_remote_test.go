package main

import (
	"example/remote/internal/logger"
	"example/remote/internal/native_messaging"
	"example/remote/internal/web_server"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type TestResponderToBrowser struct {
	messages chan web_server.OutgoingBrowserMessage
}

func NewTestResponderToBrowser() TestResponderToBrowser {
	return TestResponderToBrowser{
		messages: make(chan web_server.OutgoingBrowserMessage),
	}
}

func (resp *TestResponderToBrowser) HandleMessage(incomingMsg web_server.OutgoingBrowserMessage) {
	resp.messages <- incomingMsg
}

func TestWebServer(t *testing.T) {
	logger := logger.NewStdoutLogger()

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	// input/output formats are the same, so use another instance to simulate browser
	native := native_messaging.NewNativeMessaging[web_server.IncomingBrowserMessage, web_server.OutgoingBrowserMessage](&logger, inputReader, outputWriter)
	browser := native_messaging.NewNativeMessaging[web_server.OutgoingBrowserMessage, web_server.IncomingBrowserMessage](&logger, outputReader, inputWriter)

	ws := web_server.NewWebServer(&logger, "localhost", 5555, &native)

	browserResponder := NewTestResponderToBrowser()

	nativeDone := make(chan bool)
	browserDone := make(chan bool)
	go func() {
		native.ReadMessagesFromBrowser(&ws)
		nativeDone <- true
	}()
	go func() {
		browser.ReadMessagesFromBrowser(&browserResponder)
		browserDone <- true
	}()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{ \"query\": \"name\" }"))
	recorder := httptest.NewRecorder()

	postDone := make(chan bool)
	go func() {
		ws.HandlePost(recorder, req)
		postDone <- true
	}()

	msgToBrowser := <-browserResponder.messages
	if msgToBrowser.Query != "name" {
		t.Errorf("Invalid message sent to browser: %v", msgToBrowser.Query)
	}
	browser.SendToBrowser(web_server.IncomingBrowserMessage{Id: msgToBrowser.Id, Status: "ok", Result: "john"})

	<-postDone
	resp := recorder.Result()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}
	if string(body) != "{\"status\":\"ok\",\"result\":\"john\"}\n" {
		t.Errorf("invalid response received from web server: %v", string(body))
	}

	inputWriter.Close()
	outputWriter.Close()
	<-nativeDone
	<-browserDone
}
