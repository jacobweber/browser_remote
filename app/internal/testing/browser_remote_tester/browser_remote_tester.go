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

type TestMessageFromNativeHandler struct {
	queryListeners *mutex_map.MutexMap[string, chan shared.MessageToBrowser]
}

func NewTestMessageFromNativeHandler() TestMessageFromNativeHandler {
	return TestMessageFromNativeHandler{
		queryListeners: mutex_map.NewMap[string, chan shared.MessageToBrowser](),
	}
}

func (resp *TestMessageFromNativeHandler) HandleMessage(incomingMsg shared.MessageToBrowser) {
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
	logger                   logger.Logger
	readerFromBrowser        *io.PipeReader
	writerToNative           *io.PipeWriter
	readerFromNative         *io.PipeReader
	writerToBrowser          *io.PipeWriter
	messageReaderFromBrowser *native_messaging.NativeMessagingReader[shared.MessageFromBrowser]
	messageWriterToBrowser   *native_messaging.NativeMessagingWriter[shared.MessageToBrowser]
	messageReaderFromNative  *native_messaging.NativeMessagingReader[shared.MessageToBrowser]
	messageWriterToNative    *native_messaging.NativeMessagingWriter[shared.MessageFromBrowser]
	webServer                *web_server.WebServer
	messageFromNativeHandler *TestMessageFromNativeHandler
	readerFromBrowserDone    chan bool
	readerFromNativeDone     chan bool
}

func NewBrowserRemoteTester() BrowserRemoteTester {
	logger := logger.NewStdoutLogger()

	readerFromBrowser, writerToNative := io.Pipe()
	readerFromNative, writerToBrowser := io.Pipe()

	messageReaderFromBrowser := native_messaging.NewNativeMessagingReader[shared.MessageFromBrowser](&logger, readerFromBrowser)
	messageWriterToBrowser := native_messaging.NewNativeMessagingWriter[shared.MessageToBrowser](&logger, writerToBrowser)
	// input/output formats are the same, so use another instance to simulate browser
	messageReaderFromNative := native_messaging.NewNativeMessagingReader[shared.MessageToBrowser](&logger, readerFromNative)
	messageWriterToNative := native_messaging.NewNativeMessagingWriter[shared.MessageFromBrowser](&logger, writerToNative)

	webServer := web_server.NewWebServer(&logger, &messageWriterToBrowser)

	messageFromNativeHandler := NewTestMessageFromNativeHandler()

	readerFromBrowserDone := make(chan bool)
	readerFromNativeDone := make(chan bool)

	return BrowserRemoteTester{
		logger:                   logger,
		readerFromBrowser:        readerFromBrowser,
		writerToNative:           writerToNative,
		readerFromNative:         readerFromNative,
		writerToBrowser:          writerToBrowser,
		messageReaderFromBrowser: &messageReaderFromBrowser,
		messageWriterToBrowser:   &messageWriterToBrowser,
		messageReaderFromNative:  &messageReaderFromNative,
		messageWriterToNative:    &messageWriterToNative,
		webServer:                &webServer,
		messageFromNativeHandler: &messageFromNativeHandler,
		readerFromBrowserDone:    readerFromBrowserDone,
		readerFromNativeDone:     readerFromNativeDone,
	}
}

func (br *BrowserRemoteTester) Start() {
	go func() {
		br.messageReaderFromBrowser.Start(br.webServer)
		br.readerFromBrowserDone <- true
	}()
	go func() {
		br.messageReaderFromNative.Start(br.messageFromNativeHandler)
		br.readerFromNativeDone <- true
	}()
	go br.messageWriterToBrowser.Start()
	go br.messageWriterToNative.Start()
}

func (br *BrowserRemoteTester) SendRequestToWeb(s string) (postDone chan bool, recorder *httptest.ResponseRecorder, timeout *TestTimer) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(s))
	timeout = &TestTimer{}
	ctx := context.WithValue(req.Context(), web_server.TimerKey{}, timeout)
	req = req.WithContext(ctx)

	recorder = httptest.NewRecorder()

	postDone = make(chan bool)
	go func() {
		br.webServer.ServeHttp(recorder, req)
		postDone <- true
	}()
	return
}

func (br *BrowserRemoteTester) ListenForQueryToBrowser(s string) chan shared.MessageToBrowser {
	ch := make(chan shared.MessageToBrowser)
	br.messageFromNativeHandler.queryListeners.Set(s, ch)
	return ch
}

func (br *BrowserRemoteTester) SendResponseFromBrowser(id string, status string, result string) {
	br.messageWriterToNative.SendMessage(shared.MessageFromBrowser{Id: id, Status: status, Result: result})
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
	br.messageWriterToBrowser.Done()
	br.messageWriterToNative.Done()
}
