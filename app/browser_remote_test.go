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
	"time"
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

type TestTimer struct {
	timer chan time.Time
}

func (timer *TestTimer) StartTimer(time.Duration) <-chan time.Time {
	timer.timer = make(chan time.Time)
	return timer.timer
}

func (timer *TestTimer) FireTimer() {
	timer.timer <- time.Now()
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
	timer            *TestTimer
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

	timer := TestTimer{}

	ws := web_server.NewWebServer(&logger, "localhost", 5555, &native, &timer)

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
		timer:            &timer,
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
		br.ws.ServeHttp(br.recorder, req)
		br.postDone <- true
	}()
}

func (br *TestBrowserRemote) TimeoutWebRequest() {
	br.timer.FireTimer()
}

func (br *TestBrowserRemote) AssertBrowserReceivedQuery(s string, t *testing.T) string {
	msgToBrowser := <-br.browserResponder.messages
	if msgToBrowser.Query != s {
		t.Errorf("Invalid message sent to browser: %v", msgToBrowser.Query)
	}
	return msgToBrowser.Id
}

func (br *TestBrowserRemote) SendBrowserResponse(id string, status string, result string) {
	br.browser.SendToBrowser(web_server.IncomingBrowserMessage{Id: id, Status: status, Result: result})
}

func (br *TestBrowserRemote) AssertWebResponse(s string, t *testing.T) {
	<-br.postDone
	resp := br.recorder.Result()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}
	if string(body) != s {
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
	t.Run("responds to test", func(t *testing.T) {
		br := NewTestBrowserRemote()
		br.Start()
		br.SendWebRequest("{\"query\":\"name\"}")
		id := br.AssertBrowserReceivedQuery("name", t)
		br.SendBrowserResponse(id, "ok", "john")
		br.AssertWebResponse("{\"status\":\"ok\",\"result\":\"john\"}\n", t)
		br.Cleanup()
	})

	t.Run("ignores browser responses with invalid IDs", func(t *testing.T) {
		br := NewTestBrowserRemote()
		br.Start()
		br.SendWebRequest("{\"query\":\"name\"}")
		id := br.AssertBrowserReceivedQuery("name", t)
		br.SendBrowserResponse("xxx", "ok", "jim")
		br.SendBrowserResponse(id, "ok", "john")
		br.AssertWebResponse("{\"status\":\"ok\",\"result\":\"john\"}\n", t)
		br.Cleanup()
	})

	t.Run("responds with browser error", func(t *testing.T) {
		br := NewTestBrowserRemote()
		br.Start()
		br.SendWebRequest("{\"query\":\"name\"}")
		id := br.AssertBrowserReceivedQuery("name", t)
		br.SendBrowserResponse(id, "error", "")
		br.AssertWebResponse("{\"status\":\"error\",\"result\":\"\"}\n", t)
		br.Cleanup()
	})

	t.Run("responds with timeout error", func(t *testing.T) {
		br := NewTestBrowserRemote()
		br.Start()
		br.SendWebRequest("{\"query\":\"name\"}")
		br.AssertBrowserReceivedQuery("name", t)
		br.TimeoutWebRequest()
		br.AssertWebResponse("{\"status\":\"timeout\",\"result\":null}\n", t)
		br.Cleanup()
	})
}
