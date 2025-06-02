//go:build testing

package browser_remote_tester

import (
	"context"
	"example/remote/internal/logger"
	"example/remote/internal/mutex_map"
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
	queryListeners *mutex_map.MutexMap[string, chan web_server.OutgoingBrowserMessage]
}

func NewTestResponderToBrowser() TestResponderToBrowser {
	return TestResponderToBrowser{
		queryListeners: mutex_map.NewMap[string, chan web_server.OutgoingBrowserMessage](),
	}
}

func (resp *TestResponderToBrowser) HandleMessage(incomingMsg web_server.OutgoingBrowserMessage) {
	listener := resp.queryListeners.Get(incomingMsg.Query)
	if listener != nil {
		listener <- incomingMsg
	}
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

type BrowserRemoteTester struct {
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
}

func NewBrowserRemoteTester() BrowserRemoteTester {
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

	return BrowserRemoteTester{
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

func (br *BrowserRemoteTester) Start() {
	go func() {
		br.native.Start(br.ws)
		br.nativeDone <- true
	}()
	go func() {
		br.browser.Start(br.browserResponder)
		br.browserDone <- true
	}()
}

func (br *BrowserRemoteTester) SendRequestToWeb(s string) (postDone chan bool, recorder *httptest.ResponseRecorder, timeout *TestTimer) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(s))
	timeout = &TestTimer{}
	ctx := context.WithValue(req.Context(), web_server.TimerKey{}, timeout)
	req = req.WithContext(ctx)

	recorder = httptest.NewRecorder()

	postDone = make(chan bool)
	go func() {
		br.ws.ServeHttp(recorder, req)
		postDone <- true
	}()
	return
}

func (br *BrowserRemoteTester) ListenForQueryToBrowser(s string) chan web_server.OutgoingBrowserMessage {
	ch := make(chan web_server.OutgoingBrowserMessage)
	br.browserResponder.queryListeners.Set(s, ch)
	return ch
}

func (br *BrowserRemoteTester) SendResponseFromBrowser(id string, status string, result string) {
	br.browser.SendMessage(web_server.IncomingBrowserMessage{Id: id, Status: status, Result: result})
}

func (br *BrowserRemoteTester) AssertResponseFromWeb(postDone <-chan bool, recorder *httptest.ResponseRecorder, s string, t *testing.T) {
	<-postDone
	resp := recorder.Result()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}
	if string(body) != s {
		t.Errorf("invalid response received from web server: %v", string(body))
	}
}

func (br *BrowserRemoteTester) Cleanup() {
	br.inputWriter.Close()
	br.outputWriter.Close()
	<-br.nativeDone
	<-br.browserDone
}
