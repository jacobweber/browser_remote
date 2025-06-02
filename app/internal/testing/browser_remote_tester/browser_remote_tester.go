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
	logger                logger.Logger
	readerFromBrowser     *io.PipeReader
	writerToNative        *io.PipeWriter
	readerFromNative      *io.PipeReader
	writerToBrowser       *io.PipeWriter
	nmReaderFromBrowser   *native_messaging.NativeMessagingReader[shared.MessageFromBrowser]
	nmWriterToBrowser     *native_messaging.NativeMessagingWriter[shared.MessageToBrowser]
	nmReaderFromNative    *native_messaging.NativeMessagingReader[shared.MessageToBrowser]
	nmWriterToNative      *native_messaging.NativeMessagingWriter[shared.MessageFromBrowser]
	ws                    *web_server.WebServer
	browserResponder      *TestResponderToBrowser
	readerFromBrowserDone chan bool
	readerFromNativeDone  chan bool
}

func NewBrowserRemoteTester() BrowserRemoteTester {
	logger := logger.NewStdoutLogger()

	readerFromBrowser, writerToNative := io.Pipe()
	readerFromNative, writerToBrowser := io.Pipe()

	nmReaderFromBrowser := native_messaging.NewNativeMessagingReader[shared.MessageFromBrowser](&logger, readerFromBrowser)
	nmWriterToBrowser := native_messaging.NewNativeMessagingWriter[shared.MessageToBrowser](&logger, writerToBrowser)
	// input/output formats are the same, so use another instance to simulate browser
	nmReaderFromNative := native_messaging.NewNativeMessagingReader[shared.MessageToBrowser](&logger, readerFromNative)
	nmWriterToNative := native_messaging.NewNativeMessagingWriter[shared.MessageFromBrowser](&logger, writerToNative)

	ws := web_server.NewWebServer(&logger, &nmWriterToBrowser)

	browserResponder := NewTestResponderToBrowser()

	readerFromBrowserDone := make(chan bool)
	readerFromNativeDone := make(chan bool)

	return BrowserRemoteTester{
		logger:                logger,
		readerFromBrowser:     readerFromBrowser,
		writerToNative:        writerToNative,
		readerFromNative:      readerFromNative,
		writerToBrowser:       writerToBrowser,
		nmReaderFromBrowser:   &nmReaderFromBrowser,
		nmWriterToBrowser:     &nmWriterToBrowser,
		nmReaderFromNative:    &nmReaderFromNative,
		nmWriterToNative:      &nmWriterToNative,
		ws:                    &ws,
		browserResponder:      &browserResponder,
		readerFromBrowserDone: readerFromBrowserDone,
		readerFromNativeDone:  readerFromNativeDone,
	}
}

func (br *BrowserRemoteTester) Start() {
	go func() {
		br.nmReaderFromBrowser.Start(br.ws)
		br.readerFromBrowserDone <- true
	}()
	go func() {
		br.nmReaderFromNative.Start(br.browserResponder)
		br.readerFromNativeDone <- true
	}()
	go br.nmWriterToBrowser.Start()
	go br.nmWriterToNative.Start()
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
	br.nmWriterToNative.SendMessage(shared.MessageFromBrowser{Id: id, Status: status, Result: result})
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
	br.writerToNative.Close()
	br.writerToBrowser.Close()
	<-br.readerFromBrowserDone
	<-br.readerFromNativeDone
	br.nmWriterToBrowser.Done()
	br.nmWriterToNative.Done()
}
