//go:build testing

package browser_remote_tester

import (
	"context"
	"example/remote/internal/logger"
	"example/remote/internal/mutex_map"
	"example/remote/internal/native_messaging"
	"example/remote/internal/shared"
	"example/remote/internal/web_server"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type TestResponderToBrowser struct {
	queryListeners *mutex_map.MutexMap[string, chan shared.MessageToBrowser]
}

func NewTestResponderToBrowser() TestResponderToBrowser {
	return TestResponderToBrowser{
		queryListeners: mutex_map.NewMap[string, chan shared.MessageToBrowser](),
	}
}

func (resp *TestResponderToBrowser) HandleMessage(incomingMsg shared.MessageToBrowser) {
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
	logger            logger.Logger
	inputReader       *io.PipeReader
	inputWriter       *io.PipeWriter
	outputReader      *io.PipeReader
	outputWriter      *io.PipeWriter
	nativeReader      *native_messaging.NativeMessagingReader[shared.MessageFromBrowser]
	nativeWriter      *native_messaging.NativeMessagingWriter[shared.MessageToBrowser]
	browserReader     *native_messaging.NativeMessagingReader[shared.MessageToBrowser]
	browserWriter     *native_messaging.NativeMessagingWriter[shared.MessageFromBrowser]
	ws                *web_server.WebServer
	browserResponder  *TestResponderToBrowser
	nativeReaderDone  chan bool
	browserReaderDone chan bool
}

func NewBrowserRemoteTester() BrowserRemoteTester {
	logger := logger.NewStdoutLogger()

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	// input/output formats are the same, so use another instance to simulate browser
	nativeReader := native_messaging.NewNativeMessagingReader[shared.MessageFromBrowser](&logger, inputReader)
	nativeWriter := native_messaging.NewNativeMessagingWriter[shared.MessageToBrowser](&logger, outputWriter)
	browserReader := native_messaging.NewNativeMessagingReader[shared.MessageToBrowser](&logger, outputReader)
	browserWriter := native_messaging.NewNativeMessagingWriter[shared.MessageFromBrowser](&logger, inputWriter)

	ws := web_server.NewWebServer(&logger, &nativeWriter)

	browserResponder := NewTestResponderToBrowser()

	nativeReaderDone := make(chan bool)
	browserReaderDone := make(chan bool)

	return BrowserRemoteTester{
		logger:            logger,
		inputReader:       inputReader,
		inputWriter:       inputWriter,
		outputReader:      outputReader,
		outputWriter:      outputWriter,
		nativeReader:      &nativeReader,
		nativeWriter:      &nativeWriter,
		browserReader:     &browserReader,
		browserWriter:     &browserWriter,
		ws:                &ws,
		browserResponder:  &browserResponder,
		nativeReaderDone:  nativeReaderDone,
		browserReaderDone: browserReaderDone,
	}
}

func (br *BrowserRemoteTester) Start() {
	go func() {
		br.nativeReader.Start(br.ws)
		br.nativeReaderDone <- true
	}()
	go func() {
		br.browserReader.Start(br.browserResponder)
		br.browserReaderDone <- true
	}()
	go br.nativeWriter.Start()
	go br.browserWriter.Start()
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

func (br *BrowserRemoteTester) ListenForQueryToBrowser(s string) chan shared.MessageToBrowser {
	ch := make(chan shared.MessageToBrowser)
	br.browserResponder.queryListeners.Set(s, ch)
	return ch
}

func (br *BrowserRemoteTester) SendResponseFromBrowser(id string, status string, result string) {
	br.browserWriter.SendMessage(shared.MessageFromBrowser{Id: id, Status: status, Result: result})
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
	<-br.nativeReaderDone
	<-br.browserReaderDone
	br.nativeWriter.Done()
	br.browserWriter.Done()
}
