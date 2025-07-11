//go:build testing

package browser_remote_tester

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacobweber/browser_remote/internal/logger"
	"github.com/jacobweber/browser_remote/internal/mutex_map"
	"github.com/jacobweber/browser_remote/internal/native_messaging"
	"github.com/jacobweber/browser_remote/internal/shared"
	"github.com/jacobweber/browser_remote/internal/web_server"
)

type TestMessageFromNativeHandler struct {
	queryListeners *mutex_map.MutexMap[string, chan shared.MessageToBrowser]
}

func NewTestMessageFromNativeHandler() *TestMessageFromNativeHandler {
	return &TestMessageFromNativeHandler{
		queryListeners: mutex_map.New[string, chan shared.MessageToBrowser](),
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

func NewTestTimer() *TestTimer {
	return &TestTimer{
		timer: make(chan time.Time),
	}
}

func (timer *TestTimer) StartTimer(time.Duration) <-chan time.Time {
	return timer.timer
}

func (timer *TestTimer) FireTimer() {
	timer.timer <- time.Now()
}

type BrowserRemoteTester struct {
	logger                   *logger.Logger
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

func New() *BrowserRemoteTester {
	logger := logger.NewStdout()

	readerFromBrowser, writerToNative := io.Pipe()
	readerFromNative, writerToBrowser := io.Pipe()

	messageReaderFromBrowser := native_messaging.NewReader[shared.MessageFromBrowser](logger, readerFromBrowser, "from browser")
	messageWriterToBrowser := native_messaging.NewWriter[shared.MessageToBrowser](logger, writerToBrowser, "to browser")
	// input/output formats are the same, so use another instance to simulate browser
	messageReaderFromNative := native_messaging.NewReader[shared.MessageToBrowser](logger, readerFromNative, "from native")
	messageWriterToNative := native_messaging.NewWriter[shared.MessageFromBrowser](logger, writerToNative, "to native")

	webServer := web_server.New(logger)
	webServer.OnMessageReadyForBrowser(func(msg shared.MessageToBrowser) {
		messageWriterToBrowser.SendMessage(msg)
	})

	messageReaderFromBrowser.OnMessageRead(func(msg shared.MessageFromBrowser) {
		webServer.HandleMessageFromBrowser(msg)
	})
	messageFromNativeHandler := NewTestMessageFromNativeHandler()
	messageReaderFromNative.OnMessageRead(func(msg shared.MessageToBrowser) {
		messageFromNativeHandler.HandleMessage(msg)
	})

	readerFromBrowserDone := make(chan bool)
	readerFromNativeDone := make(chan bool)

	return &BrowserRemoteTester{
		logger:                   logger,
		readerFromBrowser:        readerFromBrowser,
		writerToNative:           writerToNative,
		readerFromNative:         readerFromNative,
		writerToBrowser:          writerToBrowser,
		messageReaderFromBrowser: messageReaderFromBrowser,
		messageWriterToBrowser:   messageWriterToBrowser,
		messageReaderFromNative:  messageReaderFromNative,
		messageWriterToNative:    messageWriterToNative,
		webServer:                webServer,
		messageFromNativeHandler: messageFromNativeHandler,
		readerFromBrowserDone:    readerFromBrowserDone,
		readerFromNativeDone:     readerFromNativeDone,
	}
}

func (br *BrowserRemoteTester) Start() {
	go func() {
		br.messageReaderFromBrowser.Start()
		br.readerFromBrowserDone <- true
	}()
	go func() {
		br.messageReaderFromNative.Start()
		br.readerFromNativeDone <- true
	}()
	go br.messageWriterToBrowser.Start()
	go br.messageWriterToNative.Start()
}

func (br *BrowserRemoteTester) SendRequestToWeb(s string) (postDone chan bool, recorder *httptest.ResponseRecorder, timeout *TestTimer) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(s))
	timeout = NewTestTimer()
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

func (br *BrowserRemoteTester) SendResponseFromBrowser(id string, status string, results []any) {
	br.messageWriterToNative.SendMessage(shared.MessageFromBrowser{Id: id, Status: status, Results: results})
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
