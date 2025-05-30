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

type TestBrowserRemote struct {
	logger           logger.Logger
	inputReader      *io.PipeReader
	inputWriter      *io.PipeWriter
	outputReader     *io.PipeReader
	outputWriter     *io.PipeWriter
	native           *native_messaging.NativeMessaging[web_server.IncomingBrowserMessage, web_server.OutgoingBrowserMessage]
	browser          *native_messaging.NativeMessaging[web_server.OutgoingBrowserMessage, web_server.IncomingBrowserMessage]
	ws               *web_server.WebServer
	browserResponder *TestResponderToBrowser
	nativeDone       chan bool
	browserDone      chan bool
	postDone         chan bool
	recorder         *httptest.ResponseRecorder
}

func NewTestBrowserRemote() TestBrowserRemote {
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

	return TestBrowserRemote{
		logger:           logger,
		inputReader:      inputReader,
		inputWriter:      inputWriter,
		outputReader:     outputReader,
		outputWriter:     outputWriter,
		native:           &native,
		browser:          &browser,
		ws:               &ws,
		browserResponder: &browserResponder,
		nativeDone:       nativeDone,
		browserDone:      browserDone,
	}
}

func (br *TestBrowserRemote) Start() {
	go func() {
		br.native.ReadMessagesFromBrowser(br.ws)
		br.nativeDone <- true
	}()
	go func() {
		br.browser.ReadMessagesFromBrowser(br.browserResponder)
		br.browserDone <- true
	}()
}

func (br *TestBrowserRemote) SendWebRequest(s string) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(s))
	br.recorder = httptest.NewRecorder()

	br.postDone = make(chan bool)
	go func() {
		br.ws.HandlePost(br.recorder, req)
		br.postDone <- true
	}()
}

func (br *TestBrowserRemote) AssertBrowserReceivedQueryAndRespond(s string, status string, result string, t *testing.T) {
	msgToBrowser := <-br.browserResponder.messages
	if msgToBrowser.Query != s {
		t.Errorf("Invalid message sent to browser: %v", msgToBrowser.Query)
	}

	br.browser.SendToBrowser(web_server.IncomingBrowserMessage{Id: msgToBrowser.Id, Status: status, Result: result})
}

func (br *TestBrowserRemote) AssertWebResponse(s string, t *testing.T) {
	<-br.postDone
	resp := br.recorder.Result()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}
	if string(body) != "{\"status\":\"ok\",\"result\":\"john\"}\n" {
		t.Errorf("invalid response received from web server: %v", string(body))
	}
}

func (br *TestBrowserRemote) Cleanup() {
	br.inputWriter.Close()
	br.outputWriter.Close()
	<-br.nativeDone
	<-br.browserDone
}

func TestWebServer(t *testing.T) {
	br := NewTestBrowserRemote()
	br.Start()
	br.SendWebRequest("{\"query\":\"name\"}")
	br.AssertBrowserReceivedQueryAndRespond("name", "ok", "john", t)
	br.AssertWebResponse("{\"status\":\"ok\",\"result\":\"john\"}\n", t)
	br.Cleanup()
}
